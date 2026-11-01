package archive

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "jogai.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func TestOpen_CreatesAPrivateArchive(t *testing.T) {
	s, path := openTemp(t)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("archive mode = %v, want 0600", info.Mode().Perm())
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm() != 0o700 {
		t.Errorf("archive dir mode = %v, want 0700", dir.Mode().Perm())
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != len(migrations) {
		t.Errorf("user_version = (%d, %v), want %d", version, err, len(migrations))
	}
}

func TestOpen_ReopensAnExistingArchive(t *testing.T) {
	s, path := openTemp(t)
	_ = s.Close()
	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_ = again.Close()
}

func TestOpen_RefusesANewerSchema(t *testing.T) {
	s, path := openTemp(t)
	if _, err := s.db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if _, err := Open(path); !errors.Is(err, ErrNewerSchema) {
		t.Errorf("Open = %v, want ErrNewerSchema", err)
	}
}

func TestOpen_MigratesAVersion1Archive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jogai.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range migrations[0] {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO messages VALUES ('u1', 's1', 1, 'user', 'hi', '', '/w', '/w', '', 0, 0, '')`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	var indexed int
	if err := s.db.QueryRow(`SELECT indexed FROM messages WHERE id = 'u1'`).Scan(&indexed); err != nil || indexed != 0 {
		t.Errorf("indexed = (%d, %v), want 0 so the message gets indexed", indexed, err)
	}
	if _, err := s.db.Exec(`INSERT INTO docs(id, kind, session_id, project, git_branch, role, title, ts, text) VALUES ('d1', 'message', 's1', '', '', 'user', '', 1, 'réglé')`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM docs_words WHERE docs_words MATCH '"regle"'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("fts hit = (%d, %v), want 1", n, err)
	}
}

func TestOpen_RereadsClaudeTranscriptsSkippedByOlderVersions(t *testing.T) {
	f := newFixture(t)
	f.appendClaude(t, "a.jsonl", line("u1", "user", "depuis l'app"))
	transcript := filepath.Join(f.home, ".claude", "projects", "-w-jogai", "a.jsonl")
	info, err := os.Stat(transcript)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "jogai.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range migrations[:2] {
		for _, stmt := range step {
			if _, err := raw.Exec(stmt); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := raw.Exec(`PRAGMA user_version = 2`); err != nil {
		t.Fatal(err)
	}
	const codex = "/home/me/.codex/sessions/b.jsonl"
	for p, pos := range map[string]int64{transcript: info.Size(), codex: 7} {
		if _, err := raw.Exec(`INSERT INTO cursors VALUES (?, ?, '', '', 0)`, p, pos); err != nil {
			t.Fatal(err)
		}
	}
	_ = raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	if _, err := s.db.Exec(`SELECT path FROM cursors`); err == nil {
		t.Error("a running older binary must not find the cursors table, or it moves the cursors past the same lines again")
	}
	cursors, err := s.cursors()
	if err != nil {
		t.Fatal(err)
	}
	if cursors[codex].Offset != 7 {
		t.Errorf("codex cursor = %d, want it untouched", cursors[codex].Offset)
	}
	res, err := s.Ingest(f.sources[:1], f.res)
	if err != nil {
		t.Fatal(err)
	}
	if res.Messages != 1 {
		t.Errorf("Messages = %d, want the transcript read again", res.Messages)
	}
}
