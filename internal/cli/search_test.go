package cli

import (
	"testing"

	"github.com/Cassidy321/jogai/internal/archive"
)

func TestSearchCmd_RefreshesTheArchiveFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeClaudeSession(t, home, noonDaysAgo(1))

	if err := (&SearchCmd{Query: []string{"work"}}).Run(); err != nil {
		t.Fatalf("search: %v", err)
	}
	path, err := archive.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	store, err := archive.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if hits, err := store.Search(archive.Query{Text: "work"}); err != nil || len(hits) != 1 {
		t.Errorf("archive after search = (%+v, %v), want the session message", hits, err)
	}
	if err := (&SearchCmd{Query: []string{"x"}, Since: "01/10/2026"}).Run(); err == nil {
		t.Error("a malformed --since must be an error")
	}
}
