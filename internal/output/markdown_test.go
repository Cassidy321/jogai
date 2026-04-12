package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/summary"
)

func TestMarkdownWrite(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 5, 20, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		Content:     "## jogai\n\nWorked on the CLI parser.",
		Sessions:    3,
	}

	if err := md.Write(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "2026-04-06.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Worked on the CLI parser") {
		t.Error("should contain summary content")
	}
	if !strings.Contains(content, "<!-- jogai-window 2026-04-05T20:00:00Z 2026-04-06T20:00:00Z -->") {
		t.Error("should contain window metadata")
	}
}

func TestMarkdownWriteCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 5, 20, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		Content:     "Summary.",
		Sessions:    5,
	}

	if err := md.Write(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "2026-04-06.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created in nested dir: %v", err)
	}
}

func TestMarkdownAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 5, 20, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		Content:     "first version",
		Sessions:    1,
	}
	if err := md.Write(s); err != nil {
		t.Fatal(err)
	}

	s.Content = "second version"
	if err := md.Write(s); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "2026-04-06.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "second version") {
		t.Error("overwrite should contain new content")
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".jogai-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestMarkdownFilename(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 12, 25, 10, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 12, 24, 10, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 12, 25, 10, 0, 0, 0, time.UTC),
		Content:     "End of year.",
		Sessions:    1,
	}

	if err := md.Write(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "2026-12-25.md")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected file %s, got error: %v", expected, err)
	}
}

func TestMarkdownRejectsConflictingOverwrite(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	first := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Content:     "scheduled recap",
	}
	if err := md.Write(first); err != nil {
		t.Fatalf("unexpected initial write error: %v", err)
	}

	second := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 16, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 16, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 16, 0, 0, 0, time.UTC),
		Content:     "manual recap",
	}
	err := md.Write(second)
	if err == nil {
		t.Fatal("expected conflicting overwrite to fail")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkdownRejectsLegacyOverwriteWithoutMetadata(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	path := filepath.Join(dir, "2026-04-11.md")
	if err := os.WriteFile(path, []byte("legacy recap\n"), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Content:     "scheduled recap",
	}

	err := md.Write(s)
	if err == nil {
		t.Fatal("expected legacy overwrite to fail")
	}
	if !strings.Contains(err.Error(), "has no window metadata") {
		t.Fatalf("unexpected error: %v", err)
	}
}
