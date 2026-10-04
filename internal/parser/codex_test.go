package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func readCodexFixture(t *testing.T, name string) []Record {
	t.Helper()
	records, _, err := (&Codex{}).ReadFrom(filepath.Join("testdata", name), Cursor{})
	if err != nil {
		t.Fatalf("ReadFrom(%s): %v", name, err)
	}
	return records
}

func TestCodexReadFrom_Happy(t *testing.T) {
	records := readCodexFixture(t, "codex_happy.jsonl")
	if len(records) != 4 {
		t.Fatalf("got %d records, want 4", len(records))
	}
	if r := records[0]; r.Role != "user" || r.Text != "regarde la PR 2132" || r.SessionID != "019d8c23-241a" || r.Cwd != "/Users/cassidy/dokaa" || r.Source != SourceCodex {
		t.Errorf("records[0] = %+v", r)
	}
	if r := records[1]; r.Role != "assistant" || r.Text != "Je regarde la PR." {
		t.Errorf("records[1] = %+v", r)
	}
}

func TestCodexReadFrom_Corrupt(t *testing.T) {
	records := readCodexFixture(t, "codex_corrupt.jsonl")
	if len(records) != 2 || records[0].Cwd != "/tmp/proj" {
		t.Errorf("records = %+v, want the 2 valid messages", records)
	}
}

func TestCodexReadFrom_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	records, _, err := (&Codex{}).ReadFrom(path, Cursor{})
	if err != nil || len(records) != 0 {
		t.Errorf("ReadFrom(empty) = (%+v, %v)", records, err)
	}
}

func TestCodexDetect(t *testing.T) {
	dir := t.TempDir()
	c := &Codex{baseDir: filepath.Join(dir, "missing")}
	if c.Detect() {
		t.Error("Detect returned true for missing dir")
	}
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if !(&Codex{baseDir: real}).Detect() {
		t.Error("Detect returned false for existing dir")
	}
}

func TestCodexFiles_WalksDayDirs(t *testing.T) {
	dir := t.TempDir()
	day := filepath.Join(dir, "2026", "04", "14")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(day, "rollout-1.jsonl")
	if err := os.WriteFile(want, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := (&Codex{baseDir: dir}).Files()
	if err != nil || len(files) != 1 || files[0] != want {
		t.Errorf("Files = (%v, %v), want [%s]", files, err, want)
	}
}
