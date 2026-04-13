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
	frontmatterDelimiter = "---"
	windowFieldPrefix    = "jogai_window:"
	legacyMetadataPrefix = "<!-- jogai-window "
	legacyMetadataSuffix = " -->"
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
	if frontmatter, ok := windowFrontmatter(s); ok {
		b.WriteString(frontmatter)
		b.WriteByte('\n')
		b.WriteByte('\n')
	}
	if title := titleLine(s); title != "" {
		b.WriteString(title)
		b.WriteByte('\n')
		b.WriteByte('\n')
	}
	b.WriteString(normalizeBody(s.Content))
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

func windowFrontmatter(s *summary.Summary) (string, bool) {
	if s.WindowStart.IsZero() || s.WindowEnd.IsZero() {
		return "", false
	}
	return fmt.Sprintf("%s\n# Managed by jogai. Do not edit manually.\n%s %q\n%s",
		frontmatterDelimiter,
		windowFieldPrefix,
		windowRange(s.WindowStart, s.WindowEnd),
		frontmatterDelimiter,
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

	first := scanner.Text()
	if first == frontmatterDelimiter {
		return parseWindowFrontmatter(scanner)
	}
	return parseLegacyWindowMetadata(first)
}

func parseWindowFrontmatter(scanner *bufio.Scanner) (time.Time, time.Time, bool) {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == frontmatterDelimiter {
			break
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, windowFieldPrefix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, windowFieldPrefix))
		raw = strings.Trim(raw, `"'`)
		return parseWindowRange(raw)
	}
	return time.Time{}, time.Time{}, false
}

func parseLegacyWindowMetadata(line string) (time.Time, time.Time, bool) {
	if !strings.HasPrefix(line, legacyMetadataPrefix) || !strings.HasSuffix(line, legacyMetadataSuffix) {
		return time.Time{}, time.Time{}, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(line, legacyMetadataPrefix), legacyMetadataSuffix)
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
	base := s.Date.Format("2006-01-02")
	switch s.Kind {
	case summary.KindSchedule:
		return fmt.Sprintf("%s.schedule.md", base)
	case summary.KindLast24h:
		return fmt.Sprintf("%s.last24h.md", base)
	default:
		return fmt.Sprintf("%s.md", base)
	}
}

func titleLine(s *summary.Summary) string {
	switch s.Kind {
	case summary.KindSchedule, summary.KindLast24h:
		return fmt.Sprintf("## %s → %s",
			s.WindowStart.Format("2006-01-02 15:04"),
			s.WindowEnd.Format("2006-01-02 15:04"),
		)
	default:
		return fmt.Sprintf("## %s", s.Date.Format("2006-01-02"))
	}
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
	if trimmed == "" {
		return false
	}
	hashes := 0
	for hashes < len(trimmed) && trimmed[hashes] == '#' {
		hashes++
	}
	if hashes == 0 || hashes > 2 {
		return false
	}
	return hashes < len(trimmed) && trimmed[hashes] == ' '
}

func windowRange(start, end time.Time) string {
	return start.Format(time.RFC3339) + ".." + end.Format(time.RFC3339)
}
