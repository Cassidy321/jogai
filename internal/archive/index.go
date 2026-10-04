package archive

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/parser"
)

type pendingMessage struct {
	rec     parser.Record
	project string
}

// Noise (injected context, command output…) stays archived in messages but
// never reaches the index: parser.Clean decides at read time.
func (s *Store) indexMessages() (int, error) {
	rows, err := s.db.Query(`SELECT id, session_id, ts, role, text, answers, project, git_branch, is_meta, is_compact, origin FROM messages WHERE indexed = 0`)
	if err != nil {
		return 0, fmt.Errorf("read unindexed messages: %w", err)
	}
	var pending []pendingMessage
	for rows.Next() {
		var p pendingMessage
		var ts int64
		var meta, compact int
		if err := rows.Scan(&p.rec.ID, &p.rec.SessionID, &ts, &p.rec.Role, &p.rec.Text, &p.rec.Answers, &p.project, &p.rec.GitBranch, &meta, &compact, &p.rec.Origin); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("read unindexed messages: %w", err)
		}
		p.rec.Timestamp = time.UnixMilli(ts)
		p.rec.IsMeta, p.rec.IsCompact = meta == 1, compact == 1
		pending = append(pending, p)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	added := 0
	for _, p := range pending {
		if text := parser.Clean(p.rec); text != "" {
			if _, err := tx.Exec(`INSERT INTO docs(id, kind, session_id, project, git_branch, role, title, ts, text)
				VALUES(?, 'message', ?, ?, ?, ?, '', ?, ?) ON CONFLICT(id) DO NOTHING`,
				p.rec.ID, p.rec.SessionID, p.project, p.rec.GitBranch, p.rec.Role, p.rec.Timestamp.UnixMilli(), text); err != nil {
				return 0, err
			}
			added++
		}
		if _, err := tx.Exec(`UPDATE messages SET indexed = 1 WHERE id = ?`, p.rec.ID); err != nil {
			return 0, err
		}
	}
	return added, tx.Commit()
}

var recapName = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.md$`)

// Recaps are indexed from the vault, not from what jogai wrote: edits made in
// Obsidian are what the user remembers.
func (s *Store) indexRecaps(dir string) (int, error) {
	if dir == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read recaps: %w", err)
	}
	known, err := s.recapFiles()
	if err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	seen := map[string]bool{}
	changed := 0
	for _, e := range entries {
		if e.IsDir() || !recapName.MatchString(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		seen[path] = true
		updated, err := indexRecapFile(tx, path, known[path])
		if err != nil {
			return changed, err
		}
		if updated {
			changed++
		}
	}
	if err := dropVanishedRecaps(tx, known, seen); err != nil {
		return changed, err
	}
	return changed, tx.Commit()
}

func indexRecapFile(tx *sql.Tx, path string, indexedMtime int64) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	mtime := info.ModTime().UnixMilli()
	if mtime == indexedMtime {
		return false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if err := replaceRecap(tx, strings.TrimSuffix(filepath.Base(path), ".md"), string(data)); err != nil {
		return false, err
	}
	_, err = tx.Exec(`INSERT INTO recap_files(path, mtime) VALUES(?, ?) ON CONFLICT(path) DO UPDATE SET mtime = excluded.mtime`, path, mtime)
	return err == nil, err
}

func dropVanishedRecaps(tx *sql.Tx, known map[string]int64, seen map[string]bool) error {
	for path := range known {
		if seen[path] {
			continue
		}
		label := strings.TrimSuffix(filepath.Base(path), ".md")
		if _, err := tx.Exec(`DELETE FROM docs WHERE session_id = ?`, "recap:"+label); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM recap_files WHERE path = ?`, path); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recapFiles() (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT path, mtime FROM recap_files`)
	if err != nil {
		return nil, fmt.Errorf("read recap files: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int64{}
	for rows.Next() {
		var path string
		var mtime int64
		if err := rows.Scan(&path, &mtime); err != nil {
			return nil, err
		}
		out[path] = mtime
	}
	return out, rows.Err()
}

func replaceRecap(tx *sql.Tx, label, content string) error {
	day, err := time.ParseInLocation(devday.LabelFormat, label, time.Local)
	if err != nil {
		return fmt.Errorf("recap %s: %w", label, err)
	}
	session := "recap:" + label
	if _, err := tx.Exec(`DELETE FROM docs WHERE session_id = ?`, session); err != nil {
		return err
	}
	for i, sec := range recapSections(content) {
		title := label
		if sec.heading != "" {
			title += " · " + sec.heading
		}
		if _, err := tx.Exec(`INSERT INTO docs(id, kind, session_id, project, git_branch, role, title, ts, text)
			VALUES(?, 'recap', ?, '', '', '', ?, ?, ?)`,
			fmt.Sprintf("%s#%d", session, i+1), session, title, day.UnixMilli(), sec.text); err != nil {
			return err
		}
	}
	return nil
}

type section struct {
	heading string
	text    string
}

// The heading line stays in the section text so a search for the project name
// finds its section.
func recapSections(content string) []section {
	var out []section
	var cur section
	var b strings.Builder
	flush := func() {
		if text := strings.TrimSpace(b.String()); text != "" {
			cur.text = text
			out = append(out, cur)
		}
		b.Reset()
	}
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "# "), strings.HasPrefix(line, "<!-- jogai-window"):
			continue
		case strings.HasPrefix(line, "## "), strings.HasPrefix(line, "### "):
			flush()
			cur = section{heading: strings.TrimSpace(strings.TrimLeft(line, "#"))}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	flush()
	return out
}
