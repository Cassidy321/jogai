package lastrun

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &Record{
		RanAt:    time.Date(2026, 4, 22, 5, 0, 0, 0, time.UTC),
		DevDay:   "2026-04-21",
		Status:   "ok",
		Warnings: []string{"codex: permission denied"},
	}
	if err := Save(r); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DevDay != "2026-04-21" {
		t.Errorf("DevDay = %q", got.DevDay)
	}
	if got.Status != "ok" {
		t.Errorf("Status = %q", got.Status)
	}
	if len(got.Warnings) != 1 {
		t.Errorf("Warnings = %v", got.Warnings)
	}
}

func TestLoad_Missing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for missing file, got %+v", got)
	}
}

func TestLoad_Corrupt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dir := filepath.Join(tmp, ".config", "jogai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last_run.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err == nil {
		t.Fatalf("expected error, got record %+v", got)
	}
}
