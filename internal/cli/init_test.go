package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestDefaultOutputDir(t *testing.T) {
	tests := []struct {
		name     string
		existing *config.Config
		fallback string
		want     string
	}{
		{"no existing", nil, "/home/jogai-recaps", "/home/jogai-recaps"},
		{"empty existing", &config.Config{}, "/home/jogai-recaps", "/home/jogai-recaps"},
		{"existing value", &config.Config{OutputDir: "/obsidian/CC"}, "/home/jogai-recaps", "/obsidian/CC"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultOutputDir(tc.existing, tc.fallback); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultDayEnd(t *testing.T) {
	tests := []struct {
		name     string
		existing *config.Config
		want     string
	}{
		{"no existing", nil, "00:00"},
		{"zero value", &config.Config{}, "00:00"},
		{"existing 05:00", &config.Config{DayEnd: config.TimeOfDay{Hour: 5, Minute: 0}}, "05:00"},
		{"existing 19:13", &config.Config{DayEnd: config.TimeOfDay{Hour: 19, Minute: 13}}, "19:13"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultDayEnd(tc.existing); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home := "/Users/cassidy"
	tests := []struct {
		in   string
		want string
	}{
		{"~/notes", "/Users/cassidy/notes"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~notes", "~notes"},
		{"~/", "/Users/cassidy"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := expandHome(tc.in, home); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProbeWriteAccess_Success(t *testing.T) {
	dir := t.TempDir()
	if err := probeWriteAccess(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == ".jogai-write-test" {
			t.Errorf("sentinel file not cleaned up")
		}
	}
}

func TestProbeWriteAccess_MissingDirectory(t *testing.T) {
	if err := probeWriteAccess(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for missing directory")
	}
}
