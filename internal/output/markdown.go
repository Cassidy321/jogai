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
	windowMarkerPrefix = "<!-- jogai-window "
	windowMarkerSuffix = " -->"
)

func NewMarkdown(dir string) *Markdown {
	return &Markdown{dir: dir}
}

func (m *Markdown) Write(s *summary.Summary) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	filename := filenameFor(s)
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
	if title := titleLine(s); title != "" {
		b.WriteString(title)
		b.WriteByte('\n')
		b.WriteByte('\n')
	}
	body := normalizeBody(s.Content)
	if body != "" {
		b.WriteString(body)
		b.WriteByte('\n')
		b.WriteByte('\n')
	}
	b.WriteString(windowMarker(s.WindowStart, s.WindowEnd))
	b.WriteByte('\n')
	return b.String()
}

func rejectConflictingOverwrite(path string, s *summary.Summary) error {
	windowStart, windowEnd, ok, err := readWindowMetadata(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read window metadata for %s: %w", path, err)
	}
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

func readWindowMetadata(path string) (time.Time, time.Time, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	defer func() { _ = f.Close() }()

	var (
		start time.Time
		end   time.Time
		found bool
	)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if s, e, ok := parseWindowMarker(scanner.Text()); ok {
			start, end, found = s, e, true
		}
	}
	if err := scanner.Err(); err != nil {
		return time.Time{}, time.Time{}, false, fmt.Errorf("scan %s: %w", path, err)
	}
	return start, end, found, nil
}

func parseWindowMarker(line string) (time.Time, time.Time, bool) {
	if !strings.HasPrefix(line, windowMarkerPrefix) || !strings.HasSuffix(line, windowMarkerSuffix) {
		return time.Time{}, time.Time{}, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(line, windowMarkerPrefix), windowMarkerSuffix)
	parts := strings.Fields(body)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, false
	}
	return parseWindowRange(parts[0] + ".." + parts[1])
}

func parseWindowRange(raw string) (time.Time, time.Time, bool) {
	parts := strings.SplitN(raw, "..", 2)
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

func filenameFor(s *summary.Summary) string {
	return s.Date.Format("2006-01-02") + ".md"
}

func titleLine(s *summary.Summary) string {
	return "# " + s.Date.Format("2006-01-02")
}

func normalizeBody(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 {
		return ""
	}

	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && isDocumentTitle(lines[i]) {
		i++
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
	}

	return strings.Join(lines[i:], "\n")
}

func isDocumentTitle(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "# ")
}

func windowMarker(start, end time.Time) string {
	return windowMarkerPrefix + start.Format(time.RFC3339) + " " + end.Format(time.RFC3339) + windowMarkerSuffix
}
