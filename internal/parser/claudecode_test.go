package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestClaudeCodeSessions(t *testing.T) {
	cc, err := NewClaudeCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cc.Detect() {
		t.Skip("Claude Code not installed, skipping")
	}

	since := time.Now().AddDate(0, -1, 0)
	sessions, err := cc.Sessions(since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) == 0 {
		t.Skip("no sessions found in the last month")
	}

	s := sessions[0]
	if s.ID == "" {
		t.Error("session ID should not be empty")
	}
	if s.Tool != "claude-code" {
		t.Errorf("expected tool 'claude-code', got '%s'", s.Tool)
	}
	if len(s.Messages) == 0 {
		t.Error("session should have at least one message")
	}
	if s.StartedAt.IsZero() {
		t.Error("session StartedAt should not be zero")
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

func TestParseSessionFromFixture(t *testing.T) {
	dir := t.TempDir()
	lines := []map[string]any{
		{
			"type":      "user",
			"sessionId": "test-123",
			"cwd":       "/Users/test/myproject",
			"timestamp": "2026-04-05T10:00:00.000Z",
			"message":   map[string]any{"role": "user", "content": "add a login page"},
		},
		{
			"type":      "assistant",
			"sessionId": "test-123",
			"cwd":       "/Users/test/myproject",
			"timestamp": "2026-04-05T10:00:05.000Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "I'll create a login page for you."}},
			},
		},
		{
			"type":      "file-history-snapshot",
			"messageId": "abc",
		},
	}

	path := filepath.Join(dir, "test-123.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		b, _ := json.Marshal(line)
		f.Write(b)
		f.Write([]byte("\n"))
	}
	f.Close()

	cc := &ClaudeCode{baseDir: "unused"}
	session, err := cc.parseSession(path, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected a session, got nil")
	}
	if session.ID != "test-123" {
		t.Errorf("expected ID 'test-123', got '%s'", session.ID)
	}
	if session.Project != "myproject" {
		t.Errorf("expected project 'myproject', got '%s'", session.Project)
	}
	if len(session.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(session.Messages))
	}
	if session.Messages[0].Content != "add a login page" {
		t.Errorf("unexpected first message: %s", session.Messages[0].Content)
	}
	if session.Messages[1].Content != "I'll create a login page for you." {
		t.Errorf("unexpected second message: %s", session.Messages[1].Content)
	}
}

func TestParseSessionSinceFiltersMessages(t *testing.T) {
	dir := t.TempDir()
	lines := []map[string]any{
		{
			"type":      "user",
			"sessionId": "test-456",
			"cwd":       "/Users/test/project",
			"timestamp": "2026-04-04T15:00:00.000Z",
			"message":   map[string]any{"role": "user", "content": "started yesterday"},
		},
		{
			"type":      "assistant",
			"sessionId": "test-456",
			"cwd":       "/Users/test/project",
			"timestamp": "2026-04-04T15:00:05.000Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "ok, working on it"}},
			},
		},
		{
			"type":      "user",
			"sessionId": "test-456",
			"cwd":       "/Users/test/project",
			"timestamp": "2026-04-05T09:00:00.000Z",
			"message":   map[string]any{"role": "user", "content": "continuing today"},
		},
		{
			"type":      "assistant",
			"sessionId": "test-456",
			"cwd":       "/Users/test/project",
			"timestamp": "2026-04-05T09:00:05.000Z",
			"message": map[string]any{
				"role":    "assistant",
				"content": []map[string]any{{"type": "text", "text": "sure, picking up where we left off"}},
			},
		},
	}

	path := filepath.Join(dir, "test-456.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		b, _ := json.Marshal(line)
		f.Write(b)
		f.Write([]byte("\n"))
	}
	f.Close()

	since, _ := time.Parse(time.RFC3339, "2026-04-05T00:00:00Z")
	cc := &ClaudeCode{baseDir: "unused"}
	session, err := cc.parseSession(path, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected a session with recent messages, got nil")
	}
	if len(session.Messages) != 2 {
		t.Errorf("expected 2 messages after since filter, got %d", len(session.Messages))
	}
	if session.Messages[0].Content != "continuing today" {
		t.Errorf("unexpected first message: %s", session.Messages[0].Content)
	}
	if session.StartedAt.Day() != 4 {
		t.Errorf("startedAt should be from the original session start (April 4), got day %d", session.StartedAt.Day())
	}
}

func TestParseSessionScannerError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-err.jsonl")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	msg := map[string]any{
		"type":      "user",
		"sessionId": "test-err",
		"cwd":       "/Users/test/project",
		"timestamp": "2026-04-05T10:00:00.000Z",
		"message":   map[string]any{"role": "user", "content": "hello"},
	}
	b, _ := json.Marshal(msg)
	f.Write(b)
	f.Write([]byte("\n"))

	// Write a line that exceeds the 10 MB scanner buffer
	f.Write([]byte(`{"type":"assistant","sessionId":"test-err","timestamp":"2026-04-05T10:00:01.000Z","message":{"role":"assistant","content":"`))
	huge := make([]byte, 11*1024*1024)
	for i := range huge {
		huge[i] = 'x'
	}
	f.Write(huge)
	f.Write([]byte(`"}}`))
	f.Write([]byte("\n"))
	f.Close()

	cc := &ClaudeCode{baseDir: "unused"}
	_, err = cc.parseSession(path, time.Time{})
	if err == nil {
		t.Error("expected an error for oversized JSONL line, got nil")
	}
}
