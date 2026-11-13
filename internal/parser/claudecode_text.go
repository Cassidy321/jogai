package parser

import (
	"encoding/json"
	"strings"
)

// Claude Code stores command output, reminders and notifications as user messages.
var systemTextPrefixes = []string{
	"<local-command-stdout>",
	"<local-command-stderr>",
	"<local-command-caveat>",
	"<bash-stdout>",
	"<bash-stderr>",
	"<system-reminder>",
	"<task-notification>",
	"<side-sessions-event>",
}

func decodeClaudeLine(raw []byte) (Record, bool) {
	var line jsonlLine
	if json.Unmarshal(raw, &line) != nil {
		return Record{}, false
	}
	switch line.Type {
	case "ai-title":
		return Record{SessionID: line.SessionID, Source: SourceClaudeCode, Title: line.AITitle}, line.AITitle != ""
	case "custom-title":
		return Record{SessionID: line.SessionID, Source: SourceClaudeCode, Title: line.CustomTitle}, line.CustomTitle != ""
	}
	if line.Type != "user" && line.Type != "assistant" {
		return Record{}, false
	}
	// SDK and `claude -p` runs come from other tools (SocaDB, scripts), not from dev
	// work. Every other entrypoint is kept: Claude Code adds surfaces without notice,
	// and a session skipped here is lost for good once it prunes the transcript.
	if strings.HasPrefix(line.Entrypoint, "sdk-") {
		return Record{}, false
	}
	r := Record{
		ID:        line.UUID,
		SessionID: line.SessionID,
		Source:    SourceClaudeCode,
		Role:      line.Message.Role,
		Text:      extractText(line.Message.Content),
		Timestamp: line.Timestamp,
		Cwd:       line.Cwd,
		GitBranch: line.GitBranch,
		IsMeta:    line.IsMeta,
		IsCompact: line.IsCompactSummary,
		Origin:    originKind(line.Origin),
		// AskUserQuestion answers are stored in the tool result, not in the message text.
		Answers: askUserAnswers(line.ToolUseResult),
	}
	return r, r.Text != "" || r.Answers != ""
}

func Clean(r Record) string {
	if r.Role == "assistant" {
		return r.Text
	}
	if r.IsMeta || r.IsCompact {
		return ""
	}
	switch r.Origin {
	case "task-notification", "peer":
		return ""
	}
	if text := cleanUserText(r.Text); text != "" {
		return text
	}
	return r.Answers
}

func originKind(raw json.RawMessage) string {
	var o struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(raw, &o) != nil {
		return ""
	}
	return o.Kind
}

func cleanUserText(text string) string {
	trimmed := strings.TrimSpace(text)
	if rest := afterLeadingReminders(trimmed); rest != trimmed {
		text, trimmed = rest, rest
	}
	for _, prefix := range systemTextPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return ""
		}
	}
	if strings.HasPrefix(trimmed, "<command-message>") || strings.HasPrefix(trimmed, "<command-name>") {
		name := tagContent(trimmed, "command-name")
		args := tagContent(trimmed, "command-args")
		return strings.TrimSpace(name + " " + args)
	}
	if strings.HasPrefix(trimmed, "<bash-input>") {
		return "! " + tagContent(trimmed, "bash-input")
	}
	return text
}

// The desktop app puts its reminders in front of what the user typed, in the
// same message: only the reminders are noise.
func afterLeadingReminders(s string) string {
	for strings.HasPrefix(s, "<system-reminder>") {
		_, rest, closed := strings.Cut(s, "</system-reminder>")
		if !closed {
			return s
		}
		s = strings.TrimSpace(rest)
	}
	return s
}

func tagContent(s, tag string) string {
	_, rest, ok := strings.Cut(s, "<"+tag+">")
	if !ok {
		return ""
	}
	inner, _, _ := strings.Cut(rest, "</"+tag+">")
	return strings.TrimSpace(inner)
}

func askUserAnswers(raw json.RawMessage) string {
	if len(raw) == 0 || raw[0] != '{' {
		return ""
	}
	var r struct {
		Questions []struct {
			Question string `json:"question"`
		} `json:"questions"`
		Answers map[string]string `json:"answers"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return ""
	}
	var lines []string
	for _, q := range r.Questions {
		if a := r.Answers[q.Question]; a != "" {
			lines = append(lines, q.Question+" → "+a)
		}
	}
	return strings.Join(lines, "\n")
}
