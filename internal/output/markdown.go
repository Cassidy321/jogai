package output

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cassidy321/jogai/internal/summary"
)

var validPeriods = map[string]bool{
	"session": true,
	"daily":   true,
	"weekly":  true,
	"monthly": true,
}

type Markdown struct {
	dir string
}

func NewMarkdown(dir string) *Markdown {
	return &Markdown{dir: dir}
}

func (m *Markdown) Write(s *summary.Summary) error {
	if !validPeriods[s.Period] {
		return fmt.Errorf("invalid period: %q", s.Period)
	}

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.md", s.Date.Format("2006-01-02"), s.Period)
	path := filepath.Join(m.dir, filename)

	content := fmt.Sprintf("# %s recap — %s\n\n%s\n",
		s.Period,
		s.Date.Format("January 2, 2006"),
		s.Content,
	)

	tmp, err := os.CreateTemp(m.dir, ".jogai-*.md")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename %s: %w", path, err)
	}

	return nil
}
