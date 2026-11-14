package archive

import (
	"strings"
	"testing"
)

func TestRefresh_IngestsAndIndexes(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl", line("u1", "user", "caffeinate garde le Mac éveillé"))
	res, err := s.Refresh(f.sources, f.res, "", 0)
	if err != nil || res.Busy || res.Ingested.Messages != 1 || res.Indexed != 1 {
		t.Fatalf("Refresh = (%+v, %v)", res, err)
	}
	if hits, err := s.Search(Query{Text: "caffeinate"}); err != nil || len(hits) != 1 {
		t.Errorf("Search after refresh = (%+v, %v)", hits, err)
	}
}

func TestIngest_WarnsWhenTranscriptsStopMatchingTheFormat(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	unknown := `{"kind":"turn","payload":"` + strings.Repeat("x", 1<<20) + `"}`
	f.appendClaude(t, "s1.jsonl", unknown, unknown, unknown, unknown, unknown, unknown)
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	if w, err := s.FormatWarning(); err != nil || !strings.Contains(w, "claude-code") {
		t.Fatalf("FormatWarning = (%q, %v), want a claude-code warning", w, err)
	}
	f.appendClaude(t, "s1.jsonl", line("u1", "user", "back to normal"))
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	if w, _ := s.FormatWarning(); w != "" {
		t.Errorf("warning should clear once messages are found again, got %q", w)
	}
}

func TestIngest_WarnsWhenSessionsHaveAnswersButNoUserMessage(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	answer := func(session string) string {
		return `{"type":"assistant","uuid":"` + session + `-a","sessionId":"` + session + `","cwd":"/w/jogai","timestamp":"2026-10-01T10:00:00Z","message":{"role":"assistant","content":[{"type":"text","text":"fait"}]}}`
	}
	for _, session := range []string{"a", "b", "c", "d"} {
		f.appendClaude(t, session+".jsonl", answer(session))
	}
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	if w, _ := s.FormatWarning(); w != "" {
		t.Fatalf("FormatWarning = %q, want nothing below %d sessions", w, muteSessions)
	}

	f.appendClaude(t, "e.jsonl", answer("e"))
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	if w, err := s.FormatWarning(); err != nil || !strings.Contains(w, "claude-code: the last 5 sessions") {
		t.Fatalf("FormatWarning = (%q, %v), want a claude-code warning", w, err)
	}

	f.appendClaude(t, "e.jsonl", `{"type":"user","uuid":"e-u","sessionId":"e","cwd":"/w/jogai","timestamp":"2026-10-01T10:05:00Z","message":{"role":"user","content":"merci"}}`)
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}
	if w, _ := s.FormatWarning(); w != "" {
		t.Errorf("warning should clear once a recent session has a user message, got %q", w)
	}
}

func TestRefresh_MasksAgainWhatOlderRulesLeftReadable(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl", line("u1", "user", "une question sans secret"))
	if _, err := s.Refresh(f.sources, f.res, "", 0); err != nil {
		t.Fatal(err)
	}
	leaked := "le déploiement lit GITHUB_TOKEN=abc123def456ghi789"
	if _, err := s.db.Exec(`UPDATE messages SET text = ? WHERE id = 'u1'`, leaked); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM docs WHERE id = 'u1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO docs(id, kind, session_id, project, git_branch, role, title, ts, text)
		VALUES ('u1', 'message', 's1', '', '', 'user', '', 1, ?)`, leaked); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE meta SET value = '1' WHERE key = 'mask_rules'`); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Refresh(f.sources, f.res, "", 0); err != nil {
		t.Fatal(err)
	}
	var text string
	if err := s.db.QueryRow(`SELECT text FROM messages WHERE id = 'u1'`).Scan(&text); err != nil || text != "le déploiement lit GITHUB_TOKEN=[secret]" {
		t.Errorf("archived text = (%q, %v), want the token masked", text, err)
	}
	if c := count(t, s, `SELECT count(*) FROM docs_words WHERE docs_words MATCH 'abc123def456ghi789'`); c != 0 {
		t.Errorf("the token is still searchable (%d hits)", c)
	}
	if c := count(t, s, `SELECT count(*) FROM docs_words WHERE docs_words MATCH 'deploiement'`); c != 1 {
		t.Errorf("fts hits = %d, want the masked message indexed again", c)
	}
}
