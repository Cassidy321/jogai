package archive

import (
	"strings"
	"testing"
)

func TestRefresh_IngestsAndIndexes(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendClaude(t, "s1.jsonl", line("u1", "user", "caffeinate garde le Mac éveillé"))
	res, err := s.Refresh(f.sources, f.res, "")
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
