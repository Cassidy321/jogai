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

type ClaudeCode struct {
	baseDir string
}

func NewClaudeCode() (*ClaudeCode, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return &ClaudeCode{
		baseDir: filepath.Join(home, ".claude", "projects"),
	}, nil
}

func (c *ClaudeCode) Name() string {
	return "claude-code"
}

func (c *ClaudeCode) Detect() bool {
	info, err := os.Stat(c.baseDir)
	return err == nil && info.IsDir()
}

func (c *ClaudeCode) Sessions(since time.Time) ([]Session, error) {
	projectDirs, err := os.ReadDir(c.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read Claude Code sessions at %s: %w", c.baseDir, err)
	}

	var sessions []Session
	for _, dir := range projectDirs {
		if !dir.IsDir() {
			continue
		}

		dirPath := filepath.Join(c.baseDir, dir.Name())
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}

			info, err := f.Info()
			if err != nil || info.ModTime().Before(since) {
				continue
			}

			session, err := c.parseSession(filepath.Join(dirPath, f.Name()), since)
			if err != nil || session == nil {
				continue
			}

			sessions = append(sessions, *session)
		}
	}

	return sessions, nil
}

type jsonlLine struct {
	Type      string    `json:"type"`
	SessionID string    `json:"sessionId"`
	Cwd       string    `json:"cwd"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func (c *ClaudeCode) parseSession(path string, since time.Time) (*Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	messages := make([]Message, 0, 128)
	var sessionID, project string
	var startedAt, endedAt time.Time

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var line jsonlLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		if line.Type != "user" && line.Type != "assistant" {
			continue
		}

		if sessionID == "" {
			sessionID = line.SessionID
			project = decodeProjectPath(line.Cwd)
			startedAt = line.Timestamp
		}
		endedAt = line.Timestamp

		if line.Timestamp.Before(since) {
			continue
		}

		text := extractText(line.Message.Content)
		if text == "" {
			continue
		}

		messages = append(messages, Message{
			Role:      line.Message.Role,
			Content:   text,
			Timestamp: line.Timestamp,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", filepath.Base(path), err)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	return &Session{
		ID:        sessionID,
		Tool:      "claude-code",
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Project:   project,
		Messages:  messages,
	}, nil
}

func extractText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	switch raw[0] {
	case '"':
		var str string
		if json.Unmarshal(raw, &str) == nil {
			return str
		}
	case '[':
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &blocks) == nil {
			var parts []string
			for _, b := range blocks {
				if b.Type == "text" && b.Text != "" {
					parts = append(parts, b.Text)
				}
			}
			return strings.Join(parts, "\n")
		}
	}

	return ""
}

func decodeProjectPath(cwd string) string {
	if cwd == "" {
		return "unknown"
	}
	return filepath.Base(cwd)
}
