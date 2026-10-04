package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	ID         string `json:"id"`
	Cwd        string `json:"cwd"`
	Originator string `json:"originator"`
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

type codexDecoder struct {
	cur Cursor
}

// Only TUI sessions are user work: SDK and `codex exec` runs are automation, and
// the Claude Code plugin's runs already show up in the Claude Code transcript.
func (d *codexDecoder) decode(raw []byte, at int64) (Record, bool) {
	var line codexLine
	if json.Unmarshal(raw, &line) != nil {
		return Record{}, false
	}
	var msg Message
	var ok bool
	switch line.Type {
	case "session_meta":
		var meta codexSessionMeta
		if json.Unmarshal(line.Payload, &meta) == nil {
			d.cur.SessionID, d.cur.Cwd = meta.ID, meta.Cwd
			d.cur.Skip = meta.Originator != "" && meta.Originator != "codex-tui"
		}
	case "turn_context":
		var tc struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(line.Payload, &tc) == nil && tc.Cwd != "" {
			d.cur.Cwd = tc.Cwd
		}
	case "event_msg":
		msg, ok = extractCodexUserEvent(line)
	case "response_item":
		msg, ok = extractCodexAssistantMessage(line)
	}
	if !ok || d.cur.Skip {
		return Record{}, false
	}
	return Record{
		// Codex lines carry no id; rollout files are append-only, so the offset is stable.
		ID:        fmt.Sprintf("%s:%d", d.cur.SessionID, at),
		SessionID: d.cur.SessionID,
		Source:    SourceCodex,
		Role:      msg.Role,
		Text:      msg.Content,
		Timestamp: msg.Timestamp,
		Cwd:       d.cur.Cwd,
	}, true
}

func (c *Codex) Files() ([]string, error) {
	var files []string
	err := filepath.WalkDir(c.baseDir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() && strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return files, err
}

func (c *Codex) ReadFrom(path string, cur Cursor) ([]Record, Cursor, error) {
	d := codexDecoder{cur: cur}
	var out []Record
	next, err := readLines(path, cur.Offset, func(raw []byte, at int64) {
		if r, ok := d.decode(raw, at); ok {
			out = append(out, r)
		}
	})
	d.cur.Offset = next
	return out, d.cur, err
}

func parseCodexSessionFile(path string) (*Session, error) {
	var d codexDecoder
	var messages []Message
	_, err := readLines(path, 0, func(raw []byte, at int64) {
		if r, ok := d.decode(raw, at); ok {
			messages = append(messages, Message{Role: r.Role, Content: r.Text, Timestamp: r.Timestamp})
		}
	})
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, nil
	}
	return &Session{
		ID:        d.cur.SessionID,
		Tool:      SourceCodex,
		StartedAt: messages[0].Timestamp,
		EndedAt:   messages[len(messages)-1].Timestamp,
		Project:   projectFromCwd(d.cur.Cwd),
		Messages:  messages,
	}, nil
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
