package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lineFrom(t *testing.T, raw string) jsonlLine {
	t.Helper()
	var l jsonlLine
	if err := json.Unmarshal([]byte(raw), &l); err != nil {
		t.Fatalf("bad fixture %s: %v", raw, err)
	}
	return l
}

func TestMessageText(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"typed user text", `{"type":"user","message":{"role":"user","content":"add a login page"}}`, "add a login page"},
		{"assistant text blocks", `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}`, "Done."},
		{"injected skill body", `{"type":"user","isMeta":true,"message":{"role":"user","content":"Base directory for this skill: x"}}`, ""},
		{"compaction summary", `{"type":"user","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued"}}`, ""},
		{"background task notification", `{"type":"user","origin":{"kind":"task-notification"},"message":{"role":"user","content":"<task-notification>done</task-notification>"}}`, ""},
		{"peer session message", `{"type":"user","origin":{"kind":"peer","from":"abc"},"message":{"role":"user","content":"hello from another session"}}`, ""},
		{"slash command keeps name and args", `{"type":"user","origin":{"kind":"human"},"message":{"role":"user","content":"<command-message>brainstorming</command-message>\n<command-name>/brainstorming</command-name>\n<command-args>refaire jogai</command-args>"}}`, "/brainstorming refaire jogai"},
		{"slash command without args", `{"type":"user","message":{"role":"user","content":"<command-name>/clear</command-name>\n<command-message>clear</command-message>\n<command-args></command-args>"}}`, "/clear"},
		{"bang command input", `{"type":"user","message":{"role":"user","content":"<bash-input>git status</bash-input>"}}`, "! git status"},
		{"bang command output", `{"type":"user","message":{"role":"user","content":"<bash-stdout>On branch main</bash-stdout><bash-stderr></bash-stderr>"}}`, ""},
		{"local command output", `{"type":"user","message":{"role":"user","content":"<local-command-stdout>Set model to sonnet</local-command-stdout>"}}`, ""},
		{"prose mentioning a tag is kept", `{"type":"user","message":{"role":"user","content":"why does <command-name> show up?"}}`, "why does <command-name> show up?"},
		{"plain tool result", `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"ok"}]},"toolUseResult":"Error: boom"}`, ""},
		{"AskUserQuestion answers", `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"x"}]},"toolUseResult":{"questions":[{"question":"Which store?"},{"question":"Keep daily files?"}],"answers":{"Which store?":"SQLite","Keep daily files?":"Yes"}}}`, "Which store? → SQLite\nKeep daily files? → Yes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messageText(lineFrom(t, tt.raw)); got != tt.want {
				t.Errorf("messageText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSessionFile_SkipsNonInteractiveLines(t *testing.T) {
	content := strings.Join([]string{
		`{"type":"user","entrypoint":"sdk-ts","sessionId":"s1","cwd":"/tmp/app","timestamp":"2026-04-05T10:00:00Z","message":{"role":"user","content":"automated prompt"}}`,
		`{"type":"assistant","entrypoint":"sdk-ts","sessionId":"s1","cwd":"/tmp/app","timestamp":"2026-04-05T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"automated answer"}]}}`,
	}, "\n")
	path := filepath.Join(t.TempDir(), "s1.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parseSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Errorf("expected an SDK-only session to be skipped, got %+v", s)
	}
}
