package archive

import (
	"database/sql"
	"errors"
	"fmt"
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
	if _, err := raw.Exec(`INSERT INTO cursors VALUES (?, ?, '', '', 0)`, transcript, info.Size()); err != nil {
		t.Fatal(err)
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
	res, err := s.Ingest(f.sources[:1], f.res)
	if err != nil {
		t.Fatal(err)
	}
	if res.Messages != 1 {
		t.Errorf("Messages = %d, want the transcript read again", res.Messages)
	}
}

func archiveAtVersion(t *testing.T, version int) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jogai.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range migrations[:version] {
		for _, stmt := range step {
			if _, err := raw.Exec(stmt); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := raw.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		t.Fatal(err)
	}
	return raw, path
}

func TestOpen_ReindexesTextTypedAfterAReminder(t *testing.T) {
	raw, path := archiveAtVersion(t, 3)
	const insert = `INSERT INTO messages(id, session_id, ts, role, text, answers, cwd, project, git_branch, is_meta, is_compact, origin, indexed)
		VALUES (?, 's1', 1, 'user', ?, '', '', '', '', 0, 0, 'human', 1)`
	for id, text := range map[string]string{
		"dropped": "<system-reminder>Today is Monday.</system-reminder>\nj'ai quoi qui tourne sur docker ?",
		"indexed": "une question déjà indexée",
	} {
		if _, err := raw.Exec(insert, id, text); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := raw.Exec(`INSERT INTO docs(id, kind, session_id, project, git_branch, role, title, ts, text)
		VALUES ('indexed', 'message', 's1', '', '', 'user', '', 1, 'une question déjà indexée')`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	if c := count(t, s, `SELECT count(*) FROM messages WHERE indexed = 0`); c != 1 {
		t.Fatalf("%d messages to index again, want only the dropped one", c)
	}
	if n, err := s.indexMessages(); err != nil || n != 1 {
		t.Fatalf("indexMessages = (%d, %v), want 1", n, err)
	}
	if c := count(t, s, `SELECT count(*) FROM docs_words WHERE docs_words MATCH 'docker'`); c != 1 {
		t.Errorf("fts hits = %d, want the typed text searchable", c)
	}
	if c := count(t, s, `SELECT count(*) FROM docs_words WHERE docs_words MATCH 'monday'`); c != 0 {
		t.Errorf("fts hits = %d, want the reminder left out", c)
	}
}

func TestOpen_RereadsCodexSessionsArchivedWithoutUserMessages(t *testing.T) {
	f := newFixture(t)
	rollout := filepath.Join(f.home, ".codex", "sessions", "2026", "11", "13", "rollout.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := `{"timestamp":"2026-11-13T14:29:35Z","type":"session_meta","payload":{"id":"cx1","cwd":"/w/jogai","originator":"codex-tui"}}
{"timestamp":"2026-11-13T14:29:36Z","type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"text","text":"ceci est un message court"}]}}}
`
	if err := os.WriteFile(rollout, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	raw, path := archiveAtVersion(t, 4)
	const claude = "/home/me/.claude/projects/-w-jogai/a.jsonl"
	for p, pos := range map[string]int{rollout: len(lines), claude: 7} {
		if _, err := raw.Exec(`INSERT INTO file_cursors VALUES (?, ?, 'cx1', '/w/jogai', 0)`, p, pos); err != nil {
			t.Fatal(err)
		}
	}
	_ = raw.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	cursors, err := s.cursors()
	if err != nil {
		t.Fatal(err)
	}
	if cursors[claude].Offset != 7 {
		t.Errorf("claude cursor = %d, want it untouched", cursors[claude].Offset)
	}
	res, err := s.Ingest(f.sources[1:], f.res)
	if err != nil {
		t.Fatal(err)
	}
	if res.Messages != 1 {
		t.Errorf("Messages = %d, want the typed message archived", res.Messages)
	}
}
