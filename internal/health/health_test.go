package health

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/lastrun"
)

func TestWarnings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Date(2026, 10, 4, 9, 0, 0, 0, time.Local)
	if err := lastrun.SaveDays(map[string]lastrun.Day{
		"2026-10-02": {Status: lastrun.StatusError, Error: "claude: Not logged in · Please run /login\ndetail"},
		"2026-10-01": {Status: lastrun.StatusRefused},
		"2026-09-01": {Status: lastrun.StatusError, Error: "too old to matter"},
		"2026-10-03": {Status: lastrun.StatusOK},
	}); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(home, ".config", "jogai")
	if err := os.WriteFile(filepath.Join(cfgDir, "last_update_error"), []byte("brew upgrade: exit 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := strings.Join(Warnings(Inputs{Now: now, LastIngest: now.Add(-96 * time.Hour), FormatWarning: "claude-code: format changed"}), "\n")
	for _, want := range []string{"2026-10-02", "Not logged in", "2026-10-01", "jogai run --day 2026-10-01", "brew upgrade: exit 1", "format changed", "not been updated since"} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings missing %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{"2026-09-01", "detail"} {
		if strings.Contains(got, bad) {
			t.Errorf("warnings should not mention %q:\n%s", bad, got)
		}
	}
}

func TestWarnings_LoginFailureNamesTheCommand(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.Local)
	days := map[string]lastrun.Day{
		"2026-11-11": {Status: lastrun.StatusError, Kind: lastrun.KindAuth, Error: "jogai: claude: Failed to authenticate"},
		"2026-11-10": {Status: lastrun.StatusError, Kind: lastrun.KindAuth, Error: "jogai: claude: Failed to authenticate"},
		"2026-11-09": {Status: lastrun.StatusError, Error: "jogai: socket closed\ndokaa: socket closed"},
	}
	tests := []struct {
		summarizer string
		want       string
	}{
		{"", "jogai cannot log in to Claude: recaps are stopped at the dev day 2026-11-10 — run `jogai login` in a terminal"},
		{"claude", "jogai cannot log in to Claude: recaps are stopped at the dev day 2026-11-10 — run `jogai login` in a terminal"},
		{"codex", "jogai cannot log in to Codex: recaps are stopped at the dev day 2026-11-10 — run `codex login` in a terminal"},
	}
	for _, tt := range tests {
		t.Setenv("HOME", t.TempDir())
		if err := lastrun.SaveDays(days); err != nil {
			t.Fatal(err)
		}
		got := Warnings(Inputs{Now: now, LastIngest: now, Summarizer: tt.summarizer})
		want := []string{tt.want, "the recap of 2026-11-09 failed and will be retried on the next run: jogai: socket closed (more in `jogai status`)"}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("summarizer %q:\n%s\nwant\n%s", tt.summarizer, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}

func TestWarnings_RefusedModelNamesTheSetting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.Local)
	if err := lastrun.SaveDays(map[string]lastrun.Day{
		"2026-11-11": {Status: lastrun.StatusError, Kind: lastrun.KindConfig, Error: "jogai: codex CLI failed"},
	}); err != nil {
		t.Fatal(err)
	}
	got := Warnings(Inputs{Now: now, LastIngest: now, Summarizer: "codex"})
	want := "Codex refuses the model set in ~/.codex/config.toml: recaps are stopped at the dev day 2026-11-11 — open Codex and pick another model with /model"
	if len(got) != 1 || got[0] != want {
		t.Errorf("Warnings = %q, want %q", got, want)
	}
}

func TestWarnings_HealthyIsSilent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	if got := Warnings(Inputs{Now: now, LastIngest: now.Add(-time.Hour)}); len(got) != 0 {
		t.Errorf("Warnings = %v, want none", got)
	}
}

func TestSystem_LeavesOutTheDevDays(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 10, 4, 9, 0, 0, 0, time.Local)
	if err := lastrun.SaveDays(map[string]lastrun.Day{"2026-10-02": {Status: lastrun.StatusError, Error: "boom"}}); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(System(Inputs{Now: now, LastIngest: now.Add(-time.Hour), FormatWarning: "format changed"}), "\n")
	if got != "format changed" {
		t.Errorf("System = %q, want only the format warning", got)
	}
}
