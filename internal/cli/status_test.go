package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
)

func TestPrintSourcesStatus_NoPanic(t *testing.T) {
	// Smoke: never panics for the various config combinations we care about.
	cases := []*config.Config{
		nil,
		{},
		{Sources: []string{"claude-code"}},
		{Sources: []string{"claude-code", "codex"}},
		{Sources: []string{"codex"}},
	}
	for _, cfg := range cases {
		printSourcesStatus(detectedSources{claudeCode: true, codex: true}, cfg)
		printSourcesStatus(detectedSources{claudeCode: false, codex: true}, cfg)
	}
}

func TestHanging(t *testing.T) {
	tests := []struct {
		name, prefix, text, want string
	}{
		{"one line", "  ✗ ", "socket closed", "  ✗ socket closed"},
		{"one line per project", "  ✗ ", "jogai: down\ndokaa: down\n", "  ✗ jogai: down\n    dokaa: down"},
		{"blank lines dropped", "! ", "a\n\n  \nb", "! a\n  b"},
		{"empty", "  ✗ ", "", "  ✗ "},
	}
	for _, tt := range tests {
		if got := hanging(tt.prefix, tt.text); got != tt.want {
			t.Errorf("%s: hanging = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDayIssue(t *testing.T) {
	one := dayIssue("2026-11-10", "socket closed")
	if one != "  ! 2026-11-10 failed, retried on the next run: socket closed" {
		t.Errorf("one line = %q", one)
	}
	several := dayIssue("2026-11-10", "jogai: down\ndokaa: down")
	want := "  ! 2026-11-10 failed, retried on the next run:\n      jogai: down\n      dokaa: down"
	if several != want {
		t.Errorf("several lines =\n%s\nwant\n%s", several, want)
	}
}

func TestTokenNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	if got := tokenNote("claude", now); got != "" {
		t.Errorf("without a token: %q, want nothing", got)
	}
	dir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude_token.json"), []byte(`{"created_at":"2026-11-12T10:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tokenNote("claude", now); got != " (token valid until 2027-11-12)" {
		t.Errorf("valid token: %q", got)
	}
	if got := tokenNote("claude", now.AddDate(1, 0, 1)); !strings.Contains(got, "expired on 2027-11-12") || !strings.Contains(got, "jogai login") {
		t.Errorf("expired token: %q", got)
	}
	if got := tokenNote("codex", now); got != "" {
		t.Errorf("codex: %q, want nothing", got)
	}
}

func TestPrintDayIssues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cfg := &config.Config{OutputDir: t.TempDir(), DayEnd: &config.TimeOfDay{Hour: 5}}
	recent := devday.Recent(now, *cfg.DayEnd, catchUpDays)

	history := map[string]lastrun.Day{}
	for _, s := range recent {
		history[s.Label] = lastrun.Day{Status: lastrun.StatusEmpty}
	}
	if err := lastrun.SaveDays(history); err != nil {
		t.Fatal(err)
	}
	if !printDayIssues(cfg, now) {
		t.Error("days without sessions must not be reported as problems")
	}

	history[recent[len(recent)-1].Label] = lastrun.Day{Status: lastrun.StatusError, Error: "socket closed"}
	if err := lastrun.SaveDays(history); err != nil {
		t.Fatal(err)
	}
	if printDayIssues(cfg, now) {
		t.Error("a failed day must be reported")
	}

	history[recent[len(recent)-1].Label] = lastrun.Day{Status: lastrun.StatusError, Kind: lastrun.KindAuth, Error: "jogai: claude: 401"}
	if err := lastrun.SaveDays(history); err != nil {
		t.Fatal(err)
	}
	if printDayIssues(cfg, now) {
		t.Error("a day stopped by the login must be reported")
	}
}

func TestPrintArchiveLine_ReportsAlerts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := archive.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	store, err := archive.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := parser.NewClaudeCode()
	if err != nil {
		t.Fatal(err)
	}
	ingest := func() {
		t.Helper()
		if _, err := store.Ingest([]parser.Source{cc}, &project.Resolver{Home: home}); err != nil {
			t.Fatal(err)
		}
	}
	ingest()
	if !printArchiveLine("") {
		t.Error("a healthy archive must not be reported")
	}

	transcript := filepath.Join(home, ".claude", "projects", "-w", "a.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte(strings.Repeat(`{"type":"unknown-format"}`+"\n", 300_000)), 0o644); err != nil {
		t.Fatal(err)
	}
	ingest()
	_ = store.Close()
	if printArchiveLine("") {
		t.Error("a format warning must be reported")
	}
}

func TestPrintArchiveLine_ReportsAlertsBeforeTheFirstRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if !printArchiveLine("") {
		t.Error("a fresh install must not be reported")
	}
	dir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last_update_error"), []byte("brew upgrade: exit 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if printArchiveLine("") {
		t.Error("a failed update must be reported even without an archive")
	}
}
