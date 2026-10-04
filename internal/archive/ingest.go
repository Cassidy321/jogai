package archive

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Cassidy321/jogai/internal/filter"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
	"github.com/Cassidy321/jogai/internal/secrets"
)

// A single pasted dump reached 267k characters; nothing past this helps a
// recap or a search.
const maxStoredChars = 32_000

type Result struct {
	Messages int
	Masked   int
}

// An unreadable file never blocks the others: its error is reported and its
// cursor stays put, so the next run retries it.
func (s *Store) Ingest(sources []parser.Source, resolver *project.Resolver) (Result, error) {
	cursors, err := s.cursors()
	if err != nil {
		return Result{}, err
	}
	in := &ingestion{store: s, resolver: resolver, projects: map[string]project.Project{}, read: map[string]int64{}, found: map[string]int{}}
	var errs []error
	for _, src := range sources {
		files, err := src.Files()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", src.Name(), err))
			continue
		}
		for _, path := range files {
			if err := in.file(src, path, cursors[path]); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, err))
			}
		}
	}
	if err := s.updateFormatWarning(in.read, in.found); err != nil {
		errs = append(errs, err)
	}
	if err := s.setMeta("last_ingest", strconv.FormatInt(time.Now().UnixMilli(), 10)); err != nil {
		errs = append(errs, err)
	}
	return in.result, errors.Join(errs...)
}

type ingestion struct {
	store    *Store
	resolver *project.Resolver
	projects map[string]project.Project
	result   Result
	read     map[string]int64
	found    map[string]int
}

func (in *ingestion) file(src parser.Source, path string, cur parser.Cursor) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() < cur.Offset {
		cur = parser.Cursor{}
	}
	if cur.Skip || info.Size() == cur.Offset {
		return nil
	}
	records, next, err := src.ReadFrom(path, cur)
	if err != nil {
		return err
	}
	in.read[src.Name()] += next.Offset - cur.Offset
	in.found[src.Name()] += len(records)
	tx, err := in.store.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, r := range records {
		if err := in.record(tx, r); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO cursors(path, pos, session_id, cwd, skip) VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET pos = excluded.pos, session_id = excluded.session_id, cwd = excluded.cwd, skip = excluded.skip`,
		path, next.Offset, next.SessionID, next.Cwd, flag(next.Skip)); err != nil {
		return err
	}
	return tx.Commit()
}

func (in *ingestion) record(tx *sql.Tx, r parser.Record) error {
	if r.Title != "" {
		_, err := tx.Exec(`INSERT INTO sessions(id, source, title) VALUES(?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET title = excluded.title`, r.SessionID, r.Source, r.Title)
		return err
	}
	proj, err := in.project(tx, r.Cwd)
	if err != nil {
		return err
	}
	text, n := secrets.Mask(r.Text)
	answers, m := secrets.Mask(r.Answers)
	ts := r.Timestamp.UnixMilli()
	res, err := tx.Exec(`INSERT OR IGNORE INTO messages(id, session_id, ts, role, text, answers, cwd, project, git_branch, is_meta, is_compact, origin)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.SessionID, ts, r.Role, filter.Truncate(text, maxStoredChars), filter.Truncate(answers, maxStoredChars),
		r.Cwd, proj.Key, r.GitBranch, flag(r.IsMeta), flag(r.IsCompact), r.Origin)
	if err != nil {
		return err
	}
	if added, err := res.RowsAffected(); err != nil || added == 0 {
		return err
	}
	in.result.Messages++
	in.result.Masked += n + m
	_, err = tx.Exec(`INSERT INTO sessions(id, source, first_at, last_at) VALUES(?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			first_at = min(coalesce(first_at, excluded.first_at), excluded.first_at),
			last_at = max(coalesce(last_at, excluded.last_at), excluded.last_at)`,
		r.SessionID, r.Source, ts, ts)
	return err
}

// Resolved once and kept: the folder may be gone the next time it's needed.
func (in *ingestion) project(tx *sql.Tx, cwd string) (project.Project, error) {
	if p, ok := in.projects[cwd]; ok {
		return p, nil
	}
	var p project.Project
	err := tx.QueryRow(`SELECT key, name FROM projects WHERE cwd = ?`, cwd).Scan(&p.Key, &p.Name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		p = in.resolver.Resolve(cwd)
		if _, err := tx.Exec(`INSERT INTO projects(cwd, key, name) VALUES(?, ?, ?)`, cwd, p.Key, p.Name); err != nil {
			return p, err
		}
	case err != nil:
		return p, err
	}
	in.projects[cwd] = p
	return p, nil
}

func (s *Store) cursors() (map[string]parser.Cursor, error) {
	rows, err := s.db.Query(`SELECT path, pos, session_id, cwd, skip FROM cursors`)
	if err != nil {
		return nil, fmt.Errorf("read cursors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]parser.Cursor{}
	for rows.Next() {
		var path string
		var c parser.Cursor
		var skip int
		if err := rows.Scan(&path, &c.Offset, &c.SessionID, &c.Cwd, &skip); err != nil {
			return nil, fmt.Errorf("read cursors: %w", err)
		}
		c.Skip = skip == 1
		out[path] = c
	}
	return out, rows.Err()
}

func (s *Store) setMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func flag(b bool) int {
	if b {
		return 1
	}
	return 0
}
