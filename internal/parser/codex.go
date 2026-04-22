package parser

import (
	"bufio"
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

func (c *Codex) Name() string { return "codex" }

func (c *Codex) Detect() bool {
	info, err := os.Stat(c.baseDir)
	return err == nil && info.IsDir()
}

func (c *Codex) Sessions(since time.Time) ([]Session, error) {
	// Walk only day folders within the window.
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	messages := make([]Message, 0, 128)
	var sessionID, project string
	var startedAt, endedAt time.Time

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var line codexLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		switch line.Type {
		case "session_meta":
			var meta codexSessionMeta
			if err := json.Unmarshal(line.Payload, &meta); err == nil {
				sessionID = meta.ID
				project = codexProject(meta.Cwd)
				if startedAt.IsZero() {
					startedAt = line.Timestamp
				}
			}
		case "event_msg":
			var ev codexEventMsg
			if err := json.Unmarshal(line.Payload, &ev); err != nil {
				continue
			}
			if ev.Type != "user_message" || ev.Message == "" {
				continue
			}
			messages = append(messages, Message{
				Role:      "user",
				Content:   ev.Message,
				Timestamp: line.Timestamp,
			})
			if startedAt.IsZero() {
				startedAt = line.Timestamp
			}
			endedAt = line.Timestamp
		case "response_item":
			var ri codexResponseItem
			if err := json.Unmarshal(line.Payload, &ri); err != nil {
				continue
			}
			if ri.Type != "message" || ri.Role != "assistant" {
				continue
			}
			var parts []string
			for _, b := range ri.Content {
				if b.Type == "output_text" && b.Text != "" {
					parts = append(parts, b.Text)
				}
			}
			text := strings.Join(parts, "\n")
			if text == "" {
				continue
			}
			messages = append(messages, Message{
				Role:      "assistant",
				Content:   text,
				Timestamp: line.Timestamp,
			})
			if startedAt.IsZero() {
				startedAt = line.Timestamp
			}
			endedAt = line.Timestamp
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", filepath.Base(path), err)
	}
	if len(messages) == 0 {
		return nil, nil
	}
	return &Session{
		ID:        sessionID,
		Tool:      "codex",
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Project:   project,
		Messages:  messages,
	}, nil
}

func codexProject(cwd string) string {
	if cwd == "" || cwd == "/" {
		return "unknown"
	}
	return filepath.Base(cwd)
}
