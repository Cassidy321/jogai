package cli

import (
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
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
}
