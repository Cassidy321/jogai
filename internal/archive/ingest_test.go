package archive

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
)

type fixture struct {
	home    string
	sources []parser.Source
	res     *project.Resolver
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cc, err := parser.NewClaudeCode()
	if err != nil {
		t.Fatal(err)
	}
	cx, err := parser.NewCodex()
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{home: home, sources: []parser.Source{cc, cx}, res: &project.Resolver{Home: home}}
}

func (f *fixture) appendClaude(t *testing.T, file string, lines ...string) {
	t.Helper()
	path := filepath.Join(f.home, ".claude", "projects", "-w-jogai", file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}
}

func line(uuid, role, text string) string {
	content := `"` + text + `"`
	if role == "assistant" {
		content = `[{"type":"text","text":"` + text + `"}]`
	}
	return `{"type":"` + role + `","uuid":"` + uuid + `","sessionId":"s1","cwd":"/w/jogai","gitBranch":"main","timestamp":"2026-10-01T10:00:00Z","message":{"role":"` + role + `","content":` + content + `}}`
}

func count(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIngest_IsIncrementalAndIdempotent(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl", line("u1", "user", "add tests"), line("u2", "assistant", "Done."))

	res, err := s.Ingest(f.sources, f.res)
	if err != nil || res.Messages != 2 {
		t.Fatalf("first ingest = (%+v, %v), want 2 messages", res, err)
	}
	if res, err := s.Ingest(f.sources, f.res); err != nil || res.Messages != 0 {
		t.Errorf("second ingest = (%+v, %v), want nothing new", res, err)
	}
	f.appendClaude(t, "s1.jsonl", line("u3", "user", "and docs"))
	if res, err := s.Ingest(f.sources, f.res); err != nil || res.Messages != 1 {
		t.Errorf("after append = (%+v, %v), want 1 message", res, err)
	}
	// A resumed session copies earlier messages, same uuids, into a new file.
	f.appendClaude(t, "s1-resumed.jsonl", line("u1", "user", "add tests"), line("u4", "user", "next"))
	if res, err := s.Ingest(f.sources, f.res); err != nil || res.Messages != 1 {
		t.Errorf("resumed copy = (%+v, %v), want only the new message", res, err)
	}
	if n := count(t, s, `SELECT count(*) FROM messages`); n != 4 {
		t.Errorf("messages = %d, want 4", n)
	}
	if n := count(t, s, `SELECT count(*) FROM sessions WHERE id = 's1' AND first_at IS NOT NULL`); n != 1 {
		t.Errorf("sessions = %d, want s1", n)
	}
}

func TestIngest_MasksSecretsAndCapsHugeMessages(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl",
		line("u1", "user", "DB_PASSWORD=hunter2hunter2 please"),
		line("u2", "user", strings.Repeat("x", maxStoredChars+1000)),
	)
	res, err := s.Ingest(f.sources, f.res)
	if err != nil || res.Masked != 1 {
		t.Fatalf("Ingest = (%+v, %v), want 1 masked secret", res, err)
	}
	var text string
	if err := s.db.QueryRow(`SELECT text FROM messages WHERE id = 'u1'`).Scan(&text); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "hunter2") {
		t.Errorf("stored text still holds the secret: %q", text)
	}
	if n := count(t, s, `SELECT length(text) FROM messages WHERE id = 'u2'`); n > maxStoredChars {
		t.Errorf("stored length = %d, want <= %d", n, maxStoredChars)
	}
}

func TestIngest_StoresProjectTitleAndBranch(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl", line("u1", "user", "hi"), `{"type":"ai-title","aiTitle":"Refaire jogai","sessionId":"s1"}`)
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	var key, branch, title string
	if err := s.db.QueryRow(`SELECT project, git_branch FROM messages WHERE id = 'u1'`).Scan(&key, &branch); err != nil {
		t.Fatal(err)
	}
	if key != "/w/jogai" || branch != "main" {
		t.Errorf("project, branch = %q, %q", key, branch)
	}
	if err := s.db.QueryRow(`SELECT title FROM sessions WHERE id = 's1'`).Scan(&title); err != nil || title != "Refaire jogai" {
		t.Errorf("title = (%q, %v)", title, err)
	}
}

func TestIngest_ConcurrentWritersDoNotDuplicate(t *testing.T) {
	f := newFixture(t)
	a, path := openTemp(t)
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	var lines []string
	for i := range 200 {
		lines = append(lines, line("u"+strconv.Itoa(i), "user", "msg"))
	}
	f.appendClaude(t, "s1.jsonl", lines...)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, s := range []*Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = s.Ingest(f.sources, f.res)
		}()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("errors = %v", errs)
	}
	if n := count(t, a, `SELECT count(*) FROM messages`); n != 200 {
		t.Errorf("messages = %d, want 200", n)
	}
}
