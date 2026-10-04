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

func messageText(line jsonlLine) string {
	if line.Type == "assistant" {
		return extractText(line.Message.Content)
	}
	if line.IsMeta || line.IsCompactSummary {
		return ""
	}
	switch originKind(line.Origin) {
	case "task-notification", "peer":
		return ""
	}
	if text := cleanUserText(extractText(line.Message.Content)); text != "" {
		return text
	}
	// AskUserQuestion answers are stored in the tool result, not in the message text.
	return askUserAnswers(line.ToolUseResult)
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
