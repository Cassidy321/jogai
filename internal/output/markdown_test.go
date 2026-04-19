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
		Date:        time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),
		Content:     "# jogai\n\nWorked on the CLI parser.",
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
	if strings.Contains(content, "# jogai") {
		t.Error("should strip model H1 heading")
	}
	if !strings.Contains(content, "<!-- jogai-window 2026-04-06T00:00:00Z 2026-04-07T00:00:00Z -->") {
		t.Error("should contain machine window marker")
	}
	if !strings.Contains(content, "# 2026-04-06") {
		t.Error("should contain managed day title")
	}
}

func TestMarkdownWriteCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),
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
		Date:        time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),
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

func TestMarkdownFilenameAlwaysDateOnly(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	// Two recaps for the same day with different windows (e.g., user changed
	// day_end between runs) should target the same filename.
	first := &summary.Summary{
		Date:        time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 12, 26, 0, 0, 0, 0, time.UTC),
		Content:     "calendar day",
		Sessions:    1,
	}
	if err := md.Write(first); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-12-25.md")); err != nil {
		t.Fatalf("expected 2026-12-25.md, got %v", err)
	}
}

func TestMarkdownRejectsConflictingOverwrite(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	first := &summary.Summary{
		Date:        time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Content:     "first window",
	}
	if err := md.Write(first); err != nil {
		t.Fatalf("unexpected initial write error: %v", err)
	}

	second := &summary.Summary{
		Date:        time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Content:     "different window same day",
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

	path := filepath.Join(dir, "2026-04-10.md")
	if err := os.WriteFile(path, []byte("legacy recap\n"), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Content:     "new recap",
	}

	err := md.Write(s)
	if err == nil {
		t.Fatal("expected legacy overwrite to fail")
	}
	if !strings.Contains(err.Error(), "has no window metadata") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkdownAcceptsSameWindowOverwrite(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	path := filepath.Join(dir, "2026-04-10.md")
	legacy := "<!-- jogai-window 2026-04-10T05:00:00Z 2026-04-11T05:00:00Z -->\n## Old title\n\nlegacy recap\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Content:     "updated recap",
	}

	if err := md.Write(s); err != nil {
		t.Fatalf("expected overwrite to succeed, got %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if !strings.Contains(string(data), "<!-- jogai-window 2026-04-10T05:00:00Z 2026-04-11T05:00:00Z -->") {
		t.Fatalf("expected rewritten file with machine marker, got:\n%s", string(data))
	}
}

func TestMarkdownUsesTrailingMarkerWhenBodyContainsOne(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	path := filepath.Join(dir, "2026-04-10.md")
	echoed := "# 2026-04-10 16:00 → 2026-04-11 16:00\n\nSummary echoed a marker: <!-- jogai-window 2026-04-09T00:00:00Z 2026-04-10T00:00:00Z -->\nmore body\n\n<!-- jogai-window 2026-04-10T16:00:00Z 2026-04-11T16:00:00Z -->\n"
	if err := os.WriteFile(path, []byte(echoed), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 10, 16, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 10, 16, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 16, 0, 0, 0, time.UTC),
		Content:     "updated recap",
	}

	if err := md.Write(s); err != nil {
		t.Fatalf("expected trailing marker to authorize overwrite, got %v", err)
	}
}

func TestMarkdownKeepsFirstSectionHeadingInBody(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
		WindowStart: time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
		Content:     "### jogai\nImplemented CLI parser.\n\n### socadb\nFixed migrations.",
	}

	if err := md.Write(s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "2026-04-13.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "### jogai") {
		t.Fatalf("expected first section heading to be preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "### socadb") {
		t.Fatalf("expected second section heading to be preserved, got:\n%s", content)
	}
}
