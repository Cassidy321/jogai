package archive

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNewerSchema = errors.New("the session archive was written by a newer jogai — update jogai")

// One entry per schema version: never edit a shipped entry, append a new one.
var migrations = [][]string{
	{
		`CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			first_at INTEGER,
			last_at INTEGER
		)`,
		`CREATE TABLE messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			ts INTEGER NOT NULL,
			role TEXT NOT NULL,
			text TEXT NOT NULL,
			answers TEXT NOT NULL,
			cwd TEXT NOT NULL,
			project TEXT NOT NULL,
			git_branch TEXT NOT NULL,
			is_meta INTEGER NOT NULL,
			is_compact INTEGER NOT NULL,
			origin TEXT NOT NULL
		)`,
		`CREATE INDEX messages_by_time ON messages(ts)`,
		`CREATE INDEX messages_by_session ON messages(session_id, ts)`,
		`CREATE TABLE projects (cwd TEXT PRIMARY KEY, key TEXT NOT NULL, name TEXT NOT NULL)`,
		`CREATE TABLE cursors (path TEXT PRIMARY KEY, pos INTEGER NOT NULL, session_id TEXT NOT NULL, cwd TEXT NOT NULL, skip INTEGER NOT NULL)`,
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	},
	{
		// Reset by a future migration when the cleaning rules change, so every
		// message is cleaned and indexed again.
		`ALTER TABLE messages ADD COLUMN indexed INTEGER NOT NULL DEFAULT 0`,
		// doc is an explicit INTEGER PRIMARY KEY because the FTS tables point at
		// it, and VACUUM may renumber an implicit rowid.
		`CREATE TABLE docs (
			doc INTEGER PRIMARY KEY,
			id TEXT NOT NULL UNIQUE,
			kind TEXT NOT NULL,
			session_id TEXT NOT NULL,
			project TEXT NOT NULL,
			git_branch TEXT NOT NULL,
			role TEXT NOT NULL,
			title TEXT NOT NULL,
			ts INTEGER NOT NULL,
			text TEXT NOT NULL
		)`,
		`CREATE INDEX docs_by_session ON docs(session_id, ts, doc)`,
		`CREATE VIRTUAL TABLE docs_words USING fts5(text, content='docs', content_rowid='doc', tokenize='unicode61 remove_diacritics 2', prefix='2 3')`,
		`CREATE VIRTUAL TABLE docs_grams USING fts5(text, content='docs', content_rowid='doc', tokenize='trigram')`,
		// No UPDATE trigger: docs are only inserted and deleted. Updating a doc in
		// place would leave the FTS tables pointing at the old text.
		`CREATE TRIGGER docs_insert AFTER INSERT ON docs BEGIN
			INSERT INTO docs_words(rowid, text) VALUES (new.doc, new.text);
			INSERT INTO docs_grams(rowid, text) VALUES (new.doc, new.text);
		END`,
		`CREATE TRIGGER docs_delete AFTER DELETE ON docs BEGIN
			INSERT INTO docs_words(docs_words, rowid, text) VALUES ('delete', old.doc, old.text);
			INSERT INTO docs_grams(docs_grams, rowid, text) VALUES ('delete', old.doc, old.text);
		END`,
		`CREATE TABLE recap_files (path TEXT PRIMARY KEY, mtime INTEGER NOT NULL)`,
	},
}

type Store struct {
	db   *sql.DB
	path string
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "jogai", "jogai.db"), nil
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create archive dir: %w", err)
	}
	// The archive keeps every prompt forever. Creating the file before SQLite
	// does makes it private, and SQLite gives -wal and -shm the same mode.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create archive: %w", err)
	}
	_ = f.Close()

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// The version is read inside the write transaction: two processes opening a
// fresh archive would otherwise both run the first migration.
func (s *Store) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("lock archive: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read archive version: %w", err)
	}
	if version > len(migrations) {
		return ErrNewerSchema
	}
	if version == len(migrations) {
		return nil
	}
	for _, step := range migrations[version:] {
		for _, stmt := range step {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("migrate archive: %w", err)
			}
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations))); err != nil {
		return fmt.Errorf("migrate archive: %w", err)
	}
	return tx.Commit()
}

type Stats struct {
	Messages   int
	Sessions   int
	LastIngest time.Time
}

func (s *Store) Stats() (Stats, error) {
	var st Stats
	if err := s.db.QueryRow(`SELECT count(*) FROM messages`).Scan(&st.Messages); err != nil {
		return st, fmt.Errorf("count messages: %w", err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM sessions WHERE first_at IS NOT NULL`).Scan(&st.Sessions); err != nil {
		return st, fmt.Errorf("count sessions: %w", err)
	}
	var ms string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'last_ingest'`).Scan(&ms)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("read last ingest: %w", err)
	}
	n, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return st, fmt.Errorf("parse last ingest: %w", err)
	}
	st.LastIngest = time.UnixMilli(n)
	return st, nil
}
