package parser

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	claudeUser      = `{"type":"user","uuid":"u1","sessionId":"s1","cwd":"/w/jogai","timestamp":"2026-04-05T10:00:00Z","message":{"role":"user","content":"add tests"}}`
	claudeAssistant = `{"type":"assistant","uuid":"u2","sessionId":"s1","cwd":"/w/jogai","timestamp":"2026-04-05T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}`
)

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeReadFrom_ResumesAfterTheLastCompleteLine(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	path := filepath.Join(cc.baseDir, "s1.jsonl")
	appendFile(t, path, claudeUser+"\n"+claudeAssistant[:40])

	records, cur, err := cc.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 1 || records[0].ID != "u1" {
		t.Fatalf("first read = (%+v, %v)", records, err)
	}
	if cur.Offset != int64(len(claudeUser)+1) {
		t.Errorf("offset = %d, want the end of the first line", cur.Offset)
	}

	appendFile(t, path, claudeAssistant[40:]+"\n")
	records, _, err = cc.ReadFrom(path, cur)
	if err != nil || len(records) != 1 || records[0].ID != "u2" {
		t.Fatalf("resumed read = (%+v, %v), want the completed line only", records, err)
	}
}

func TestClaudeReadFrom_FallsBackToOffsetWithoutUUID(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	path := filepath.Join(cc.baseDir, "s1.jsonl")
	appendFile(t, path, `{"type":"user","sessionId":"s1","timestamp":"2026-04-05T10:00:00Z","message":{"role":"user","content":"old format"}}`+"\n")
	records, _, err := cc.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 1 || records[0].ID != "s1:0" {
		t.Fatalf("records = (%+v, %v), want id s1:0", records, err)
	}
}

func TestClaudeFiles_SkipsSubagents(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	main := filepath.Join(cc.baseDir, "-w-jogai", "s1.jsonl")
	sub := filepath.Join(cc.baseDir, "-w-jogai", "s1", "subagents", "agent-1.jsonl")
	for _, p := range []string{main, sub} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		appendFile(t, p, claudeUser+"\n")
	}
	files, err := cc.Files()
	if err != nil || len(files) != 1 || files[0] != main {
		t.Errorf("Files = (%v, %v), want only %s", files, err, main)
	}
}
