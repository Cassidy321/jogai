package archive

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Cassidy321/jogai/internal/filter"
)

type Query struct {
	Text           string
	Project        string
	Kind           string
	Since, Until   time.Time
	Limit          int
	ExcludeSession string
}

type Hit struct {
	ID        string
	Kind      string
	SessionID string
	Project   string
	Branch    string
	Role      string
	Title     string
	Time      time.Time
	Snippet   string
}

func (s *Store) Search(q Query) ([]Hit, error) {
	terms := searchTerms(q.Text)
	if len(terms) == 0 {
		return nil, nil
	}
	limit := q.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	hits, err := s.searchIndex("docs_words", ftsMatch(terms, 2, " ", true), q, limit)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 && len(terms) > 1 {
		if hits, err = s.searchIndex("docs_words", ftsMatch(terms, 2, " OR ", true), q, limit); err != nil {
			return nil, err
		}
	}
	// unicode61 indexes rejectConflictingOverwrite as one word: substrings of
	// identifiers are only reachable through the trigram index.
	if len(hits) < limit {
		grams, err := s.searchIndex("docs_grams", ftsMatch(terms, 3, " ", false), q, limit)
		if err != nil {
			return nil, err
		}
		hits = appendNew(hits, grams, limit)
	}
	return hits, nil
}

// FTS5 has its own query language: the text is reduced to plain terms, each
// quoted, so "c'est", "DOK-358" or "a:b" can never be a syntax error.
func searchTerms(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-' && r != '.'
	})
	var out []string
	for _, f := range fields {
		if strings.IndexFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) >= 0 {
			out = append(out, f)
		}
	}
	return out
}

func ftsMatch(terms []string, minLen int, sep string, prefix bool) string {
	var parts []string
	for _, t := range terms {
		if utf8.RuneCountInString(t) < minLen {
			continue
		}
		p := `"` + t + `"`
		if prefix {
			p += "*"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, sep)
}

func appendNew(hits, more []Hit, limit int) []Hit {
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.ID] = true
	}
	for _, h := range more {
		if len(hits) == limit {
			break
		}
		if !seen[h.ID] {
			hits = append(hits, h)
			seen[h.ID] = true
		}
	}
	return hits
}

// Until recap docs carry a project, a project filter keeps the recap sections
// whose heading names it. bm25 is negative (lower is better): the factor
// favors recaps.
func (s *Store) searchIndex(table, match string, q Query, limit int) ([]Hit, error) {
	if match == "" {
		return nil, nil
	}
	since, until := int64(math.MinInt64), int64(math.MaxInt64)
	if !q.Since.IsZero() {
		since = q.Since.UnixMilli()
	}
	if !q.Until.IsZero() {
		until = q.Until.UnixMilli()
	}
	rows, err := s.db.Query(`SELECT d.id, d.kind, d.session_id,
			coalesce((SELECT name FROM projects WHERE key = d.project LIMIT 1), ''),
			d.git_branch, d.role, d.title, d.ts, snippet(`+table+`, 0, '«', '»', '…', 24)
		FROM `+table+` JOIN docs d ON d.doc = `+table+`.rowid
		WHERE `+table+` MATCH ?
			AND (? = '' OR d.kind = ?)
			AND d.ts >= ? AND d.ts < ?
			AND d.session_id != ?
			AND (? = ''
				OR d.project IN (SELECT key FROM projects WHERE name = ? COLLATE NOCASE)
				OR (d.kind = 'recap' AND d.title LIKE '%' || ? || '%'))
		ORDER BY bm25(`+table+`) * (CASE d.kind WHEN 'recap' THEN 1.2 ELSE 1.0 END)
		LIMIT ?`,
		match, q.Kind, q.Kind, since, until, q.ExcludeSession, q.Project, q.Project, q.Project, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var hits []Hit
	for rows.Next() {
		var h Hit
		var ts int64
		if err := rows.Scan(&h.ID, &h.Kind, &h.SessionID, &h.Project, &h.Branch, &h.Role, &h.Title, &ts, &h.Snippet); err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
		h.Time = time.UnixMilli(ts)
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

type Page struct {
	Text string
	Next string
}

// A "more" id is prefixed with from: so paging never rewinds to the question
// again, which would return the same page forever.
func (s *Store) Read(id string, budget int) (Page, error) {
	id, resume := strings.CutPrefix(id, "from:")
	var doc, ts int64
	var kind, session string
	err := s.db.QueryRow(`SELECT doc, kind, session_id, ts FROM docs WHERE id = ?`, id).Scan(&doc, &kind, &session, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, fmt.Errorf("nothing archived with id %q", id)
	}
	if err != nil {
		return Page{}, err
	}
	if kind == "message" && !resume {
		// Start at the question that opened the exchange: an answer read without
		// its question is easy to misread.
		err := s.db.QueryRow(`SELECT doc, ts FROM docs WHERE session_id = ? AND role = 'user' AND (ts < ? OR (ts = ? AND doc <= ?))
			ORDER BY ts DESC, doc DESC LIMIT 1`, session, ts, ts, doc).Scan(&doc, &ts)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Page{}, err
		}
	}
	rows, err := s.db.Query(`SELECT id, role, title, ts, text FROM docs WHERE session_id = ? AND (ts > ? OR (ts = ? AND doc >= ?)) ORDER BY ts, doc`,
		session, ts, ts, doc)
	if err != nil {
		return Page{}, err
	}
	defer func() { _ = rows.Close() }()

	var b strings.Builder
	b.WriteString(s.sessionHeader(kind, session))
	page := Page{}
	wrote := false
	for rows.Next() {
		var did, role, title, text string
		var dts int64
		if err := rows.Scan(&did, &role, &title, &dts, &text); err != nil {
			return Page{}, err
		}
		block := formatDoc(kind, role, title, time.UnixMilli(dts), text)
		if wrote && b.Len()+len(block) > budget {
			page.Next = "from:" + did
			break
		}
		b.WriteString(filter.Truncate(block, budget))
		wrote = true
	}
	page.Text = b.String()
	if page.Next != "" {
		page.Text += fmt.Sprintf("\n[more: read id %q]\n", page.Next)
	}
	return page, rows.Err()
}

func (s *Store) sessionHeader(kind, session string) string {
	if kind == "recap" {
		return "Recap " + strings.TrimPrefix(session, "recap:") + "\n"
	}
	var title, project string
	_ = s.db.QueryRow(`SELECT coalesce((SELECT title FROM sessions WHERE id = ?), ''),
		coalesce((SELECT p.name FROM docs d JOIN projects p ON p.key = d.project WHERE d.session_id = ? LIMIT 1), '')`,
		session, session).Scan(&title, &project)
	if title == "" {
		title = session
	}
	if project == "" {
		project = "no project"
	}
	return fmt.Sprintf("Session %q · %s\n", title, project)
}

func formatDoc(kind, role, title string, at time.Time, text string) string {
	if kind == "recap" {
		return fmt.Sprintf("── %s ──\n%s\n", title, text)
	}
	return fmt.Sprintf("── %s · %s ──\n%s\n", at.Format("2006-01-02 15:04"), role, text)
}

func FormatHits(hits []Hit) string {
	var b strings.Builder
	for i, h := range hits {
		fmt.Fprintf(&b, "%d. %s\n   %s\n   id: %s\n", i+1, h.header(), strings.Join(strings.Fields(h.Snippet), " "), h.ID)
	}
	return b.String()
}

func (h Hit) header() string {
	if h.Kind == "recap" {
		return "recap " + h.Title
	}
	project := h.Project
	if project == "" {
		project = "no project"
	}
	parts := []string{h.Time.Format("2006-01-02 15:04"), project}
	if h.Branch != "" && h.Branch != "HEAD" {
		parts = append(parts, h.Branch)
	}
	return strings.Join(append(parts, h.Role), " · ")
}
