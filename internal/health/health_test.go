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

func TestWarnings_HealthyIsSilent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	if got := Warnings(Inputs{Now: now, LastIngest: now.Add(-time.Hour)}); len(got) != 0 {
		t.Errorf("Warnings = %v, want none", got)
	}
}
