package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decoded(t *testing.T, raw string) Record {
	t.Helper()
	r, _ := decodeClaudeLine([]byte(raw))
	return r
}

func TestClean(t *testing.T) {
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
			if got := Clean(decoded(t, tt.raw)); got != tt.want {
				t.Errorf("Clean = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeClaudeLine(t *testing.T) {
	r, ok := decodeClaudeLine([]byte(`{"type":"user","uuid":"u1","sessionId":"s1","cwd":"/w/jogai","gitBranch":"feat/x","timestamp":"2026-04-05T10:00:00Z","isMeta":true,"origin":{"kind":"human"},"message":{"role":"user","content":"Base directory for this skill"}}`))
	if !ok {
		t.Fatal("a meta line is archived even though Clean drops it")
	}
	if r.ID != "u1" || r.SessionID != "s1" || r.Cwd != "/w/jogai" || r.GitBranch != "feat/x" || !r.IsMeta || r.Origin != "human" || r.Source != SourceClaudeCode {
		t.Errorf("record = %+v", r)
	}

	title, ok := decodeClaudeLine([]byte(`{"type":"ai-title","aiTitle":"Refaire jogai","sessionId":"s1"}`))
	if !ok || title.Title != "Refaire jogai" || title.SessionID != "s1" || title.Role != "" {
		t.Errorf("title = (%+v, %v)", title, ok)
	}

	if _, ok := decodeClaudeLine([]byte(`{"type":"assistant","entrypoint":"sdk-cli","message":{"role":"assistant","content":[{"type":"text","text":"x"}]}}`)); ok {
		t.Error("lines from SDK and `claude -p` runs must be skipped")
	}
	if _, ok := decodeClaudeLine([]byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash"}]}}`)); ok {
		t.Error("a line without text or answers has nothing to archive")
	}
}

func TestClaudeReadFrom_SkipsNonInteractiveLines(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	path := filepath.Join(cc.baseDir, "s1.jsonl")
	content := strings.Join([]string{
		`{"type":"user","uuid":"a","entrypoint":"sdk-ts","sessionId":"s1","cwd":"/tmp/app","timestamp":"2026-04-05T10:00:00Z","message":{"role":"user","content":"automated prompt"}}`,
		`{"type":"assistant","uuid":"b","entrypoint":"sdk-ts","sessionId":"s1","cwd":"/tmp/app","timestamp":"2026-04-05T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"automated answer"}]}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	records, _, err := cc.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 0 {
		t.Errorf("ReadFrom = (%+v, %v), want SDK lines skipped", records, err)
	}
}
