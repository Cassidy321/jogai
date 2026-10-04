package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIndexMessages_CleansAtReadTime(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl",
		line("u1", "user", "comment on a réglé les prompts TCC"),
		`{"type":"user","uuid":"u2","isMeta":true,"sessionId":"s1","cwd":"/w/jogai","timestamp":"2026-10-01T10:00:01Z","message":{"role":"user","content":"Base directory for this skill"}}`,
	)
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	n, err := s.indexMessages()
	if err != nil || n != 1 {
		t.Fatalf("indexMessages = (%d, %v), want 1 doc (the meta line is noise)", n, err)
	}
	if again, err := s.indexMessages(); err != nil || again != 0 {
		t.Errorf("second pass = (%d, %v), want 0", again, err)
	}
	if c := count(t, s, `SELECT count(*) FROM messages WHERE indexed = 0`); c != 0 {
		t.Errorf("%d messages left unindexed", c)
	}
	if c := count(t, s, `SELECT count(*) FROM docs_words WHERE docs_words MATCH '"regle" "tcc"'`); c != 1 {
		t.Errorf("fts hits = %d, want 1", c)
	}
}

func writeRecap(t *testing.T, dir, label, body string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(dir, label+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestRecapSections(t *testing.T) {
	got := recapSections("# 2026-09-25\n\n> ⚠ codex: boom\n\n### Dokaa\n\nDOK-358 embed\n\n### jogai\n\nretry\n\n<!-- jogai-window a b -->\n")
	if len(got) != 3 {
		t.Fatalf("sections = %+v, want intro + 2", got)
	}
	if got[1].heading != "Dokaa" || !strings.Contains(got[1].text, "DOK-358") || strings.Contains(got[2].text, "jogai-window") {
		t.Errorf("sections = %+v", got)
	}
	if one := recapSections("# 2026-09-03\n\n**Joja** merge et soutenance\n"); len(one) != 1 || one[0].heading != "" {
		t.Errorf("a recap without headings is one section, got %+v", one)
	}
}

func TestIndexRecaps_FollowsTheVault(t *testing.T) {
	s, _ := openTemp(t)
	dir := t.TempDir()
	t0 := time.Date(2026, 9, 26, 5, 0, 0, 0, time.Local)
	writeRecap(t, dir, "2026-09-25", "# 2026-09-25\n\n### Dokaa\n\nembed iframe\n", t0)
	writeRecap(t, dir, "2026-09-24", "# 2026-09-24\n\nretry policy\n", t0)
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("not a recap"), 0o644); err != nil {
		t.Fatal(err)
	}

	if n, err := s.indexRecaps(dir); err != nil || n != 2 {
		t.Fatalf("first pass = (%d, %v), want 2 files", n, err)
	}
	if n, err := s.indexRecaps(dir); err != nil || n != 0 {
		t.Errorf("unchanged vault = (%d, %v), want 0", n, err)
	}
	writeRecap(t, dir, "2026-09-25", "# 2026-09-25\n\n### Dokaa\n\nembed widget\n", t0.Add(time.Hour))
	if n, err := s.indexRecaps(dir); err != nil || n != 1 {
		t.Errorf("edited recap = (%d, %v), want 1", n, err)
	}
	if c := count(t, s, `SELECT count(*) FROM docs_words WHERE docs_words MATCH '"iframe"'`); c != 0 {
		t.Errorf("the old text of an edited recap is still indexed")
	}
	if err := os.Remove(filepath.Join(dir, "2026-09-24.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.indexRecaps(dir); err != nil {
		t.Fatal(err)
	}
	if c := count(t, s, `SELECT count(*) FROM docs WHERE kind = 'recap'`); c != 1 {
		t.Errorf("recap docs = %d, want only the remaining file's section", c)
	}
}

func TestIndexRecaps_AttachesSectionsToTheirProject(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl", line("u1", "user", "hi"))
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeRecap(t, dir, "2026-10-01", "# 2026-10-01\n\n## jogai\n\nflock everywhere\n\n## Hors projet\n\nune question\n", time.Now())
	if _, err := s.indexRecaps(dir); err != nil {
		t.Fatal(err)
	}
	var project string
	if err := s.db.QueryRow(`SELECT project FROM docs WHERE kind = 'recap' AND title LIKE '%jogai'`).Scan(&project); err != nil || project != "/w/jogai" {
		t.Errorf("jogai section project = (%q, %v)", project, err)
	}
	if hits, err := s.Search(Query{Text: "flock", Project: "jogai"}); err != nil || len(hits) != 1 {
		t.Errorf("project-filtered search = (%+v, %v)", hits, err)
	}
	if c := count(t, s, `SELECT count(*) FROM docs WHERE kind = 'recap' AND project = '' AND title LIKE '%Hors projet'`); c != 1 {
		t.Errorf("the Hors projet section must stay without project")
	}
}
