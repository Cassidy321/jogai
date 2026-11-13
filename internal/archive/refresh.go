package archive

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/lock"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
)

// Claude Code changes its transcript format without notice. Megabytes of new
// lines that yield no message mean jogai stopped understanding them, silently.
const formatDriftBytes = 5 << 20

type RefreshResult struct {
	Busy     bool
	Ingested Result
	Indexed  int
	Recaps   int
}

// Every Claude session starts its own `jogai mcp`: only one refreshes at a
// time. Searches pass wait 0 and use what is already archived; a recap run
// waits, since recapping from a half-imported archive writes partial recaps.
func (s *Store) Refresh(sources []parser.Source, resolver *project.Resolver, recapDir string, wait time.Duration) (RefreshResult, error) {
	release, err := lock.TryFor(filepath.Join(filepath.Dir(s.path), "refresh.lock"), wait)
	if errors.Is(err, lock.ErrBusy) {
		return RefreshResult{Busy: true}, nil
	}
	if err != nil {
		return RefreshResult{}, err
	}
	defer release()
	var res RefreshResult
	var errIngest, errIndex, errRecaps error
	res.Ingested, errIngest = s.Ingest(sources, resolver)
	res.Indexed, errIndex = s.indexMessages()
	res.Recaps, errRecaps = s.indexRecaps(recapDir)
	return res, errors.Join(errIngest, errIndex, errRecaps)
}

func (s *Store) updateFormatWarning(read map[string]int64, found map[string]int) error {
	var drifted []string
	anyFound := false
	for name, n := range read {
		if n >= formatDriftBytes && found[name] == 0 {
			drifted = append(drifted, name)
		}
		anyFound = anyFound || found[name] > 0
	}
	sort.Strings(drifted)
	var problems []string
	if len(drifted) > 0 {
		problems = append(problems, fmt.Sprintf("%s: megabytes of new transcripts on %s but no message found — the format may have changed; update jogai",
			strings.Join(drifted, ", "), time.Now().Format("2006-01-02")))
	}
	mute, err := s.sourcesWithoutUserMessages()
	if err != nil {
		return err
	}
	if len(mute) > 0 {
		problems = append(problems, fmt.Sprintf("%s: the last %d sessions have answers but nothing typed by the user — the format may have changed; update jogai",
			strings.Join(mute, ", "), muteSessions))
	}
	switch {
	case len(problems) > 0:
		return s.setMeta("format_warning", strings.Join(problems, "; "))
	case anyFound:
		_, err := s.db.Exec(`DELETE FROM meta WHERE key = 'format_warning'`)
		return err
	}
	return nil
}

// A format change can lose one side of the conversation only (Codex 0.155
// moved the user's messages): megabytes are still read and answers still
// found, so the byte check above stays silent. An interactive session always
// holds something from the user.
const muteSessions = 5

func (s *Store) sourcesWithoutUserMessages() ([]string, error) {
	rows, err := s.db.Query(`SELECT source FROM (
			SELECT id, source, row_number() OVER (PARTITION BY source ORDER BY first_at DESC) AS age
			FROM sessions WHERE first_at IS NOT NULL
		) recent
		WHERE age <= ?
		GROUP BY source
		HAVING count(*) = ? AND sum(EXISTS (SELECT 1 FROM messages m WHERE m.session_id = recent.id AND m.role = 'user')) = 0
		ORDER BY source`, muteSessions, muteSessions)
	if err != nil {
		return nil, fmt.Errorf("check recent sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			return nil, err
		}
		out = append(out, source)
	}
	return out, rows.Err()
}

func (s *Store) FormatWarning() (string, error) {
	var w string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'format_warning'`).Scan(&w)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return w, err
}
