package parser

import (
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
	return SourceClaudeCode
}

func (c *ClaudeCode) Detect() bool {
	info, err := os.Stat(c.baseDir)
	return err == nil && info.IsDir()
}

// Subagent transcripts sit one level deeper (<session>/subagents/) and are left
// out: what they found already shows up in the parent session.
func (c *ClaudeCode) Files() ([]string, error) {
	return filepath.Glob(filepath.Join(c.baseDir, "*", "*.jsonl"))
}

func (c *ClaudeCode) ReadFrom(path string, cur Cursor) ([]Record, Cursor, error) {
	var out []Record
	next, err := readLines(path, cur.Offset, func(raw []byte, at int64) {
		r, ok := decodeClaudeLine(raw)
		if !ok {
			return
		}
		if r.ID == "" && r.Title == "" {
			r.ID = fmt.Sprintf("%s:%d", r.SessionID, at)
		}
		out = append(out, r)
	})
	cur.Offset = next
	return out, cur, err
}

type jsonlLine struct {
	Type             string    `json:"type"`
	UUID             string    `json:"uuid"`
	GitBranch        string    `json:"gitBranch"`
	AITitle          string    `json:"aiTitle"`
	SessionID        string    `json:"sessionId"`
	Cwd              string    `json:"cwd"`
	Timestamp        time.Time `json:"timestamp"`
	Entrypoint       string    `json:"entrypoint"`
	IsMeta           bool      `json:"isMeta"`
	IsCompactSummary bool      `json:"isCompactSummary"`
	// Raw on purpose: if Claude Code changes their shape, a typed field would
	// fail the whole line's unmarshal and silently drop the message.
	Origin        json.RawMessage `json:"origin"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
	Message       struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
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
