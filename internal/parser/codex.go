package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Codex struct {
	baseDir string
}

func NewCodex() (*Codex, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return &Codex{baseDir: filepath.Join(home, ".codex", "sessions")}, nil
}

func (c *Codex) Name() string { return SourceCodex }

func (c *Codex) Detect() bool {
	info, err := os.Stat(c.baseDir)
	return err == nil && info.IsDir()
}

func (c *Codex) Sessions(since time.Time) ([]Session, error) {
	start := since.Truncate(24 * time.Hour)
	now := time.Now().UTC()
	var sessions []Session
	for d := start; !d.After(now.Add(24 * time.Hour)); d = d.Add(24 * time.Hour) {
		dir := filepath.Join(c.baseDir, d.Format("2006"), d.Format("01"), d.Format("02"))
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read codex day dir %s: %w", dir, err)
		}
		for _, f := range entries {
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			info, err := f.Info()
			if err != nil || info.ModTime().Before(since) {
				continue
			}
			session, err := parseCodexSessionFile(filepath.Join(dir, f.Name()))
			if err != nil || session == nil {
				continue
			}
			filtered := filterMessages(session.Messages, since)
			if len(filtered) == 0 {
				continue
			}
			session.Messages = filtered
			sessions = append(sessions, *session)
		}
	}
	return sessions, nil
}

type codexLine struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID  string `json:"id"`
	Cwd string `json:"cwd"`
}

type codexEventMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type codexResponseItem struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func parseCodexSessionFile(path string) (*Session, error) {
	messages := make([]Message, 0, 128)
	var sessionID, project string
	var startedAt, endedAt time.Time

	err := scanJSONL(path, func(raw []byte) {
		var line codexLine
		if err := json.Unmarshal(raw, &line); err != nil {
			return
		}
		switch line.Type {
		case "session_meta":
			extractCodexSessionMeta(line, &sessionID, &project, &startedAt)
		case "event_msg":
			if msg, ok := extractCodexUserEvent(line); ok {
				messages = append(messages, msg)
				if startedAt.IsZero() {
					startedAt = line.Timestamp
				}
				endedAt = line.Timestamp
			}
		case "response_item":
			if msg, ok := extractCodexAssistantMessage(line); ok {
				messages = append(messages, msg)
				if startedAt.IsZero() {
					startedAt = line.Timestamp
				}
				endedAt = line.Timestamp
			}
		}
	})
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, nil
	}
	return &Session{
		ID:        sessionID,
		Tool:      SourceCodex,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Project:   project,
		Messages:  messages,
	}, nil
}

func extractCodexSessionMeta(line codexLine, sessionID, project *string, startedAt *time.Time) {
	var meta codexSessionMeta
	if err := json.Unmarshal(line.Payload, &meta); err != nil {
		return
	}
	*sessionID = meta.ID
	*project = projectFromCwd(meta.Cwd)
	if startedAt.IsZero() {
		*startedAt = line.Timestamp
	}
}

func extractCodexUserEvent(line codexLine) (Message, bool) {
	var ev codexEventMsg
	if err := json.Unmarshal(line.Payload, &ev); err != nil {
		return Message{}, false
	}
	if ev.Type != "user_message" || ev.Message == "" {
		return Message{}, false
	}
	return Message{
		Role:      "user",
		Content:   ev.Message,
		Timestamp: line.Timestamp,
	}, true
}

func extractCodexAssistantMessage(line codexLine) (Message, bool) {
	var ri codexResponseItem
	if err := json.Unmarshal(line.Payload, &ri); err != nil {
		return Message{}, false
	}
	if ri.Type != "message" || ri.Role != "assistant" {
		return Message{}, false
	}
	var parts []string
	for _, b := range ri.Content {
		if b.Type == "output_text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	text := strings.Join(parts, "\n")
	if text == "" {
		return Message{}, false
	}
	return Message{
		Role:      "assistant",
		Content:   text,
		Timestamp: line.Timestamp,
	}, true
}
