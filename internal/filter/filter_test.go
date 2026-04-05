package filter

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Cassidy321/jogai/internal/parser"
)

func TestReduceTruncatesLongAssistantMessages(t *testing.T) {
	long := strings.Repeat("x", MaxAssistantChars+500)
	sessions := []parser.Session{
		{
			ID:   "s1",
			Tool: "claude-code",
			Messages: []parser.Message{
				{Role: "user", Content: "short message"},
				{Role: "assistant", Content: long},
			},
		},
	}

	result := Reduce(sessions)
	msg := result[0].Messages[1]

	if !strings.Contains(msg.Content, "[...]") {
		t.Error("truncated message should contain [...]")
	}
	if !strings.HasPrefix(msg.Content, "xxx") {
		t.Error("should keep the beginning of the message")
	}
	if !strings.HasSuffix(msg.Content, "xxx") {
		t.Error("should keep the end of the message")
	}
}

func TestReduceKeepsUserMessagesIntact(t *testing.T) {
	long := strings.Repeat("a", MaxAssistantChars+500)
	sessions := []parser.Session{
		{
			ID:   "s1",
			Tool: "claude-code",
			Messages: []parser.Message{
				{Role: "user", Content: long},
			},
		},
	}

	result := Reduce(sessions)
	if result[0].Messages[0].Content != long {
		t.Error("user messages should never be truncated")
	}
}

func TestReduceKeepsAllMessages(t *testing.T) {
	var messages []parser.Message
	for i := 0; i < 100; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages = append(messages, parser.Message{
			Role:    role,
			Content: strings.Repeat("a", 500),
		})
	}

	sessions := []parser.Session{
		{ID: "s1", Tool: "claude-code", Messages: messages},
	}

	result := Reduce(sessions)
	if len(result[0].Messages) != 100 {
		t.Errorf("expected all 100 messages kept, got %d", len(result[0].Messages))
	}
}

func TestReduceDropsEmptySessions(t *testing.T) {
	sessions := []parser.Session{
		{ID: "s1", Tool: "claude-code", Messages: []parser.Message{}},
		{ID: "s2", Tool: "claude-code", Messages: []parser.Message{
			{Role: "user", Content: "hello"},
		}},
	}

	result := Reduce(sessions)
	if len(result) != 1 {
		t.Errorf("expected 1 session (empty dropped), got %d", len(result))
	}
	if result[0].ID != "s2" {
		t.Errorf("expected session s2, got %s", result[0].ID)
	}
}

func TestReducePreservesMetadata(t *testing.T) {
	start := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)

	sessions := []parser.Session{
		{
			ID:        "s1",
			Tool:      "claude-code",
			StartedAt: start,
			EndedAt:   end,
			Project:   "myproject",
			Messages:  []parser.Message{{Role: "user", Content: "hi"}},
		},
	}

	result := Reduce(sessions)
	s := result[0]
	if s.ID != "s1" || s.Tool != "claude-code" || s.Project != "myproject" {
		t.Error("metadata should be preserved")
	}
	if !s.StartedAt.Equal(start) || !s.EndedAt.Equal(end) {
		t.Error("timestamps should be preserved")
	}
}

func TestReduceShortSessionUntouched(t *testing.T) {
	sessions := []parser.Session{
		{
			ID:   "s1",
			Tool: "claude-code",
			Messages: []parser.Message{
				{Role: "user", Content: "add a login page"},
				{Role: "assistant", Content: "Sure, I'll create a login page."},
			},
		},
	}

	result := Reduce(sessions)
	if result[0].Messages[0].Content != "add a login page" {
		t.Error("short messages should not be modified")
	}
	if result[0].Messages[1].Content != "Sure, I'll create a login page." {
		t.Error("short messages should not be modified")
	}
}

func TestReduceHandlesUTF8(t *testing.T) {
	long := strings.Repeat("é", MaxAssistantChars+100)
	sessions := []parser.Session{
		{
			ID:   "s1",
			Tool: "claude-code",
			Messages: []parser.Message{
				{Role: "assistant", Content: long},
			},
		},
	}

	result := Reduce(sessions)
	msg := result[0].Messages[0]

	if !strings.HasPrefix(msg.Content, "éé") {
		t.Error("should preserve valid UTF-8 at the start")
	}
	if !strings.HasSuffix(msg.Content, "éé") {
		t.Error("should preserve valid UTF-8 at the end")
	}
	if !strings.Contains(msg.Content, "[...]") {
		t.Error("truncated message should contain [...]")
	}
}

func TestCollapseCodeBlocks(t *testing.T) {
	input := "I'll create the file.\n```tsx\nexport function Login() {\n  return <div>Login</div>\n}\n```\nDone, it's ready."
	got := collapseCodeBlocks(input)
	expected := "I'll create the file.\n[code block: tsx, 3 lines]\nDone, it's ready."
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestCollapseCodeBlocksNoLang(t *testing.T) {
	input := "Here:\n```\nsome code\n```\nEnd."
	got := collapseCodeBlocks(input)
	expected := "Here:\n[code block: 1 lines]\nEnd."
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestCollapseCodeBlocksMultiple(t *testing.T) {
	input := "First:\n```go\nfunc main() {}\n```\nThen:\n```sql\nSELECT 1;\nSELECT 2;\n```\nDone."
	got := collapseCodeBlocks(input)
	expected := "First:\n[code block: go, 1 lines]\nThen:\n[code block: sql, 2 lines]\nDone."
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestCollapseCodeBlocksNoCode(t *testing.T) {
	input := "Just plain text, no code."
	got := collapseCodeBlocks(input)
	if got != input {
		t.Errorf("expected unchanged text, got: %s", got)
	}
}

func TestCollapseCodeBlocksUnclosed(t *testing.T) {
	input := "Start:\n```go\nfunc broken() {"
	got := collapseCodeBlocks(input)
	if got != input {
		t.Errorf("unclosed block should be kept as-is, got: %s", got)
	}
}

func TestCollapseCodeBlocksInlineBackticks(t *testing.T) {
	input := "Use ```npm install``` to install dependencies."
	got := collapseCodeBlocks(input)
	if got != input {
		t.Errorf("inline backticks should not be treated as code blocks, got: %s", got)
	}
}

func TestReduceCollapsesCodeThenTruncates(t *testing.T) {
	code := "```go\n" + strings.Repeat("x := 1\n", 500) + "```"
	content := "I'll fix the bug.\n" + code + "\nThe bug is fixed."

	sessions := []parser.Session{
		{
			ID:   "s1",
			Tool: "claude-code",
			Messages: []parser.Message{
				{Role: "assistant", Content: content},
			},
		},
	}

	result := Reduce(sessions)
	msg := result[0].Messages[0]

	if strings.Contains(msg.Content, "x := 1") {
		t.Error("code should be collapsed, not present in output")
	}
	if !strings.Contains(msg.Content, "[code block:") {
		t.Error("should contain code block marker")
	}
	if !strings.Contains(msg.Content, "I'll fix the bug") {
		t.Error("text before code should be preserved")
	}
	if !strings.Contains(msg.Content, "The bug is fixed") {
		t.Error("text after code should be preserved")
	}
}

func TestCollapseCodeBlocksNested(t *testing.T) {
	input := "Here's how:\n````md\n```go\nfunc main() {}\n```\n````\nDone."
	got := collapseCodeBlocks(input)
	expected := "Here's how:\n[code block: md, 3 lines]\nDone."
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestCollapseCodeBlocksEmpty(t *testing.T) {
	input := "Before:\n```go\n```\nAfter."
	got := collapseCodeBlocks(input)
	expected := "Before:\n[code block: go, 0 lines]\nAfter."
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestMarkerRuneLen(t *testing.T) {
	if utf8.RuneCountInString(marker) != markerRuneLen {
		t.Errorf("markerRuneLen (%d) does not match actual rune count (%d)", markerRuneLen, utf8.RuneCountInString(marker))
	}
}

func TestTruncatedMessageRespectsMaxRunes(t *testing.T) {
	for _, content := range []string{
		strings.Repeat("x", MaxAssistantChars+500),
		strings.Repeat("é", MaxAssistantChars+500),
		strings.Repeat("你", MaxAssistantChars+500),
	} {
		sessions := []parser.Session{
			{
				ID:   "s1",
				Tool: "claude-code",
				Messages: []parser.Message{
					{Role: "assistant", Content: content},
				},
			},
		}

		result := Reduce(sessions)
		msg := result[0].Messages[0]
		runeCount := utf8.RuneCountInString(msg.Content)

		if runeCount > MaxAssistantChars {
			t.Errorf("truncated message has %d runes, expected <= %d", runeCount, MaxAssistantChars)
		}
	}
}
