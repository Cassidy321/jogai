package output

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/summary"
)

type Markdown struct {
	dir string
}

const (
	windowMetadataPrefix = "<!-- jogai-window "
	windowMetadataSuffix = " -->"
)

func NewMarkdown(dir string) *Markdown {
	return &Markdown{dir: dir}
}

func (m *Markdown) Write(s *summary.Summary) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	filename := fmt.Sprintf("%s.md", s.Date.Format("2006-01-02"))
	path := filepath.Join(m.dir, filename)

	if err := rejectConflictingOverwrite(path, s); err != nil {
		return err
	}

	content := renderContent(s)

	tmp, err := os.CreateTemp(m.dir, ".jogai-*.md")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s: %w", path, err)
	}

	return nil
}

func renderContent(s *summary.Summary) string {
	var b strings.Builder
	if line, ok := windowMetadataLine(s); ok {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(s.Content)
	b.WriteByte('\n')
	return b.String()
}

func rejectConflictingOverwrite(path string, s *summary.Summary) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}

	windowStart, windowEnd, ok := readWindowMetadata(path)
	if !ok {
		return fmt.Errorf(
			"refusing to overwrite %s — existing recap has no window metadata; delete it manually if you want to replace it",
			path,
		)
	}
	if s.WindowStart.Equal(windowStart) && s.WindowEnd.Equal(windowEnd) {
		return nil
	}
	return fmt.Errorf(
		"refusing to overwrite %s — existing recap covers %s to %s, new recap covers %s to %s",
		path,
		windowStart.Format(time.RFC3339),
		windowEnd.Format(time.RFC3339),
		s.WindowStart.Format(time.RFC3339),
		s.WindowEnd.Format(time.RFC3339),
	)
}

func windowMetadataLine(s *summary.Summary) (string, bool) {
	if s.WindowStart.IsZero() || s.WindowEnd.IsZero() {
		return "", false
	}
	return fmt.Sprintf("%s%s %s%s",
		windowMetadataPrefix,
		s.WindowStart.Format(time.RFC3339),
		s.WindowEnd.Format(time.RFC3339),
		windowMetadataSuffix,
	), true
}

func readWindowMetadata(path string) (time.Time, time.Time, bool) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return time.Time{}, time.Time{}, false
	}
	return parseWindowMetadata(scanner.Text())
}

func parseWindowMetadata(line string) (time.Time, time.Time, bool) {
	if !strings.HasPrefix(line, windowMetadataPrefix) || !strings.HasSuffix(line, windowMetadataSuffix) {
		return time.Time{}, time.Time{}, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(line, windowMetadataPrefix), windowMetadataSuffix)
	parts := strings.Fields(body)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, false
	}
	start, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	end, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}
