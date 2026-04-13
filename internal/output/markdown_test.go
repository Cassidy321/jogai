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
		Kind:        summary.KindDay,
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
	if strings.Contains(content, "## jogai") {
		t.Error("should replace model heading with managed title")
	}
	if !strings.Contains(content, `jogai_window: "2026-04-05T20:00:00Z..2026-04-06T20:00:00Z"`) {
		t.Error("should contain YAML window metadata")
	}
	if !strings.Contains(content, "## 2026-04-06") {
		t.Error("should contain managed day title")
	}
}

func TestMarkdownWriteCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 6, 20, 0, 0, 0, time.UTC),
		Kind:        summary.KindDay,
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
		Kind:        summary.KindDay,
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

func TestMarkdownFilenameByKind(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	day := &summary.Summary{
		Date:        time.Date(2026, 12, 25, 10, 0, 0, 0, time.UTC),
		Kind:        summary.KindDay,
		WindowStart: time.Date(2026, 12, 24, 10, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 12, 25, 10, 0, 0, 0, time.UTC),
		Content:     "End of year.",
		Sessions:    1,
	}
	schedule := &summary.Summary{
		Date:        time.Date(2026, 12, 25, 10, 0, 0, 0, time.UTC),
		Kind:        summary.KindSchedule,
		WindowStart: time.Date(2026, 12, 24, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 12, 25, 5, 0, 0, 0, time.UTC),
		Content:     "scheduled",
		Sessions:    1,
	}
	last24h := &summary.Summary{
		Date:        time.Date(2026, 12, 25, 10, 0, 0, 0, time.UTC),
		Kind:        summary.KindLast24h,
		WindowStart: time.Date(2026, 12, 24, 10, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 12, 25, 10, 0, 0, 0, time.UTC),
		Content:     "last24h",
		Sessions:    1,
	}

	for _, s := range []*summary.Summary{day, schedule, last24h} {
		if err := md.Write(s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	expected := []string{
		filepath.Join(dir, "2026-12-25.md"),
		filepath.Join(dir, "2026-12-25.schedule.md"),
		filepath.Join(dir, "2026-12-25.last24h.md"),
	}
	for _, path := range expected {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s, got error: %v", path, err)
		}
	}
}

func TestMarkdownRejectsConflictingOverwrite(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	first := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Kind:        summary.KindSchedule,
		WindowStart: time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Content:     "scheduled recap",
	}
	if err := md.Write(first); err != nil {
		t.Fatalf("unexpected initial write error: %v", err)
	}

	second := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 16, 0, 0, 0, time.UTC),
		Kind:        summary.KindSchedule,
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

func TestMarkdownAllowsDifferentKindsSameDay(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	day := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Kind:        summary.KindDay,
		WindowStart: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC),
		Content:     "calendar day",
	}
	schedule := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Kind:        summary.KindSchedule,
		WindowStart: time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Content:     "scheduled window",
	}
	last24h := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 16, 0, 0, 0, time.UTC),
		Kind:        summary.KindLast24h,
		WindowStart: time.Date(2026, 4, 10, 16, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 4, 11, 16, 0, 0, 0, time.UTC),
		Content:     "manual window",
	}

	for _, s := range []*summary.Summary{day, schedule, last24h} {
		if err := md.Write(s); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	for _, name := range []string{"2026-04-11.md", "2026-04-11.schedule.md", "2026-04-11.last24h.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
}

func TestMarkdownRejectsLegacyOverwriteWithoutMetadata(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	path := filepath.Join(dir, "2026-04-11.schedule.md")
	if err := os.WriteFile(path, []byte("legacy recap\n"), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Kind:        summary.KindSchedule,
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

func TestMarkdownAcceptsLegacyCommentForSameWindow(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	path := filepath.Join(dir, "2026-04-11.schedule.md")
	legacy := "<!-- jogai-window 2026-04-10T05:00:00Z 2026-04-11T05:00:00Z -->\n## Old title\n\nlegacy recap\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		Kind:        summary.KindSchedule,
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
	if !strings.Contains(string(data), `jogai_window: "2026-04-10T05:00:00Z..2026-04-11T05:00:00Z"`) {
		t.Fatalf("expected updated frontmatter, got:\n%s", string(data))
	}
}

func TestMarkdownKeepsFirstSectionHeadingInBody(t *testing.T) {
	dir := t.TempDir()
	md := NewMarkdown(dir)

	s := &summary.Summary{
		Date:        time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
		Kind:        summary.KindDay,
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
