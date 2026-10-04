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
}

func decodeClaudeLine(raw []byte) (Record, bool) {
	var line jsonlLine
	if json.Unmarshal(raw, &line) != nil {
		return Record{}, false
	}
	if line.Type == "ai-title" {
		return Record{SessionID: line.SessionID, Source: SourceClaudeCode, Title: line.AITitle}, line.AITitle != ""
	}
	if line.Type != "user" && line.Type != "assistant" {
		return Record{}, false
	}
	// SDK and `claude -p` runs come from other tools (SocaDB, scripts), not from dev work.
	if line.Entrypoint != "" && line.Entrypoint != "cli" {
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
