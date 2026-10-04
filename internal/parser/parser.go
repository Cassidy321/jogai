package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	SourceClaudeCode = "claude-code"
	SourceCodex      = "codex"
)

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	ID        string    `json:"id"`
	Tool      string    `json:"tool"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Project   string    `json:"project"`
	Messages  []Message `json:"messages"`
}

type Parser interface {
	Name() string
	Detect() bool
	Sessions(since time.Time) ([]Session, error)
}

func scanJSONL(path string, perLine func(line []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		perLine(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", filepath.Base(path), err)
	}
	return nil
}

// A session being written ends with a partial line: stop before it so the next
// read starts from that line instead of skipping it.
func readLines(path string, offset int64, fn func(line []byte, at int64)) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return offset, err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	r := bufio.NewReaderSize(f, 64*1024)
	pos := offset
	for {
		line, err := r.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return pos, nil
		}
		if err != nil {
			return pos, fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		fn(line[:len(line)-1], pos)
		pos += int64(len(line))
	}
}

func projectFromCwd(cwd string) string {
	if cwd == "" || cwd == "/" {
		return "unknown"
	}
	return filepath.Base(cwd)
}
