package parser

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeCodeDetect(t *testing.T) {
	cc, err := NewClaudeCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cc.Detect() {
		t.Skip("Claude Code not installed, skipping")
	}
}

func TestExtractTextString(t *testing.T) {
	raw := json.RawMessage(`"hello world"`)
	got := extractText(raw)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", got)
	}
}

func TestExtractTextBlocks(t *testing.T) {
	raw := json.RawMessage(`[{"type":"text","text":"first"},{"type":"thinking","text":"ignore"},{"type":"text","text":"second"}]`)
	got := extractText(raw)
	if got != "first\nsecond" {
		t.Errorf("expected 'first\\nsecond', got '%s'", got)
	}
}

func TestExtractTextEmpty(t *testing.T) {
	got := extractText(nil)
	if got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}

func TestClaudeReadFrom_Fixture(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	path := filepath.Join(cc.baseDir, "test-123.jsonl")
	appendFile(t, path, strings.Join([]string{
		`{"type":"user","uuid":"u1","sessionId":"test-123","cwd":"/Users/test/myproject","timestamp":"2026-04-05T10:00:00.000Z","message":{"role":"user","content":"add a login page"}}`,
		`{"type":"assistant","uuid":"u2","sessionId":"test-123","cwd":"/Users/test/myproject","timestamp":"2026-04-05T10:00:05.000Z","message":{"role":"assistant","content":[{"type":"text","text":"I'll create a login page for you."}]}}`,
		`{"type":"file-history-snapshot","messageId":"abc"}`,
	}, "\n")+"\n")

	records, _, err := cc.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 2 {
		t.Fatalf("ReadFrom = (%+v, %v), want the two messages", records, err)
	}
	if r := records[0]; r.SessionID != "test-123" || r.Cwd != "/Users/test/myproject" || Clean(r) != "add a login page" {
		t.Errorf("first record = %+v", r)
	}
	if got := Clean(records[1]); got != "I'll create a login page for you." {
		t.Errorf("second record = %q", got)
	}
}
