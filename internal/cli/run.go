package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/claudetoken"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/health"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/recap"
	"github.com/Cassidy321/jogai/internal/scheduler"
	"github.com/Cassidy321/jogai/internal/summary"
	"github.com/Cassidy321/jogai/internal/update"
)

const catchUpDays = 14

type RunCmd struct {
	Day   string `name:"day" help:"Recap a specific dev day (YYYY-MM-DD), even if it was already recapped."`
	Force bool   `help:"With --day, replace a recap even if it was edited by hand."`

	// Legacy v0.4 flags, kept hidden for schedule backward compatibility.
	Scheduled bool   `kong:"hidden"`
	At        string `kong:"hidden"`
}

func (c *RunCmd) Run() error {
	release, err := config.AcquireLock()
	// Not an error: when a manual run repairs the plist, the reload starts the
	// job through RunAtLoad and it lands here while the manual run recaps.
	if errors.Is(err, config.ErrLocked) {
		logf("Another jogai run is in progress — nothing to do.")
		return nil
	}
	if err != nil {
		return err
	}
	defer release()

	recorded, err := c.run()
	if !recorded && scheduler.InJob() {
		noteRun(err)
	}
	return err
}

// A scheduled run that fails before any dev day, or finds nothing to recap,
// leaves no trace in the day history: without this record a stopped schedule
// and a run that fails every night at its first step both stay silent.
func noteRun(failure error) {
	r, _ := lastrun.Load()
	if r == nil || r.DevDay == "" || failure != nil {
		r = &lastrun.Record{Status: lastrun.StatusOK}
	}
	r.RanAt = time.Now()
	if failure != nil {
		r.Status, r.Error = lastrun.StatusError, failure.Error()
	}
	_ = lastrun.Save(r)
}

func (c *RunCmd) run() (recorded bool, err error) {
	cfg, err := config.Load()
	if err != nil {
		return false, err
	}
	if cfg.DayEnd == nil {
		return false, fmt.Errorf("dev day boundary not configured — run 'jogai init' to set it")
	}
	repairSchedule()
	if changed, err := registerMCP(stableExecutable()); err != nil {
		logErrf("⚠ could not register jogai in Claude Code: %v", err)
	} else if changed {
		logf("Claude Code can now search your past sessions (MCP server %q).", mcpName)
	}
	defer maybeUpdate(cfg)
	return c.recapPending(cfg)
}

func maybeUpdate(cfg *config.Config) {
	if !scheduler.InJob() || !cfg.AutoUpdateEnabled() {
		return
	}
	attempted, err := update.Daily(context.Background())
	switch {
	case err != nil:
		logErrf("⚠ auto-update failed: %v", err)
	case attempted:
		logf("Checked Homebrew for a newer jogai.")
	}
}

func repairSchedule() {
	s, err := scheduler.New()
	if err != nil {
		return
	}
	changed, err := s.Repair()
	if err != nil {
		logErrf("⚠ could not update the schedule: %v", err)
		return
	}
	if changed {
		logf("Schedule updated to match this version of jogai.")
	}
}

func (c *RunCmd) recapPending(cfg *config.Config) (recorded bool, err error) {
	env, err := openArchive(cfg)
	if err != nil {
		return false, fmt.Errorf("session archive: %w", err)
	}
	defer func() { _ = env.store.Close() }()
	warnings := refreshArchive(env)
	sizer := selectSummarizer(cfg.Summarizer)
	if err := sizer.CheckCLI(); err != nil {
		return false, err
	}

	days := loadDays()
	spans, err := c.spans(time.Now(), *cfg.DayEnd, days, recapExists(cfg.OutputDir), len(warnings) == 0)
	if err != nil {
		return false, err
	}
	if len(spans) == 0 {
		logf("Every recent dev day is already recapped.")
		return false, nil
	}
	logf("Recapping %s", describeSpans(spans))

	guarded := &guardedWriter{md: output.NewMarkdown(cfg.OutputDir), days: days, force: c.Force, hashes: map[string]string{}}
	p := &recap.Pipeline{
		Archive:    env.store,
		Summarizer: summary.NewRetry(sizer),
		Writer:     guarded,
		Warnings:   warnings,
	}
	results := p.Run(context.Background(), spans)

	failed := record(days, results, warnings, guarded.hashes)
	if hint := stoppedHint(days, results, cfg.Summarizer); hint != "" {
		logErrf("%s", hint)
	}
	if err := lastrun.SaveDays(days); err != nil {
		logErrf("⚠ could not save the run history: %v", err)
	}
	saveLastRun(results, warnings)
	if failed > 0 {
		return true, fmt.Errorf("%d dev day(s) failed — see the errors above", failed)
	}
	return true, nil
}

func (c *RunCmd) spans(now time.Time, dayEnd config.TimeOfDay, days map[string]lastrun.Day, exists func(label string) bool, allRead bool) ([]devday.Span, error) {
	if c.Day == "" {
		return pendingSpans(devday.Recent(now, dayEnd, catchUpDays), days, exists, allRead), nil
	}
	span, err := c.daySpan(now, dayEnd)
	if err != nil {
		return nil, err
	}
	return []devday.Span{span}, nil
}

func (c *RunCmd) daySpan(now time.Time, dayEnd config.TimeOfDay) (devday.Span, error) {
	date, err := time.ParseInLocation(devday.LabelFormat, c.Day, now.Location())
	if err != nil {
		return devday.Span{}, fmt.Errorf("invalid --day date %q — expected YYYY-MM-DD", c.Day)
	}
	start, end, label := devday.FromDate(date, dayEnd)
	if end.After(now) {
		return devday.Span{}, fmt.Errorf("dev day %s is not yet complete — window ends at %s", c.Day, end.Format("2006-01-02 15:04"))
	}
	return devday.Span{Start: start, End: end, Label: label}, nil
}

// A recap file without a recorded outcome predates the day history: count it
// as done rather than regenerating two weeks of recaps after an upgrade.
// A partial day was recapped while some sessions could not be read: it is
// redone only once they all can, or every run would rewrite it for nothing.
func pendingSpans(candidates []devday.Span, days map[string]lastrun.Day, exists func(label string) bool, allRead bool) []devday.Span {
	var out []devday.Span
	for _, s := range candidates {
		d, recorded := days[s.Label]
		retry := d.NeedsRetry() || (d.Status == lastrun.StatusPartial && allRead)
		if recorded && !retry {
			continue
		}
		if !recorded && exists(s.Label) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func recapExists(dir string) func(label string) bool {
	return func(label string) bool {
		_, err := os.Stat(filepath.Join(dir, label+".md"))
		return err == nil
	}
}

func loadDays() map[string]lastrun.Day {
	days, err := lastrun.LoadDays()
	if err != nil {
		logErrf("⚠ %v — treating recent dev days as not yet recapped", err)
		return map[string]lastrun.Day{}
	}
	return days
}

func describeSpans(spans []devday.Span) string {
	if len(spans) == 1 {
		s := spans[0]
		return fmt.Sprintf("dev day %s (%s → %s)", s.Label, s.Start.Format("Jan 02 15:04"), s.End.Format("Jan 02 15:04"))
	}
	labels := make([]string, len(spans))
	for i, s := range spans {
		labels[i] = s.Label
	}
	return fmt.Sprintf("%d dev days: %s", len(spans), strings.Join(labels, ", "))
}

func record(days map[string]lastrun.Day, results []recap.Day, warnings []string, hashes map[string]string) int {
	failed := 0
	now := time.Now()
	for _, r := range results {
		d := lastrun.Day{UpdatedAt: now, Hash: hashes[r.Span.Label]}
		if d.Hash == "" {
			d.Hash = days[r.Span.Label].Hash
		}
		switch {
		case errors.Is(r.Err, errEditedByHand):
			d.Status = lastrun.StatusOK
			logf("dev day %s: kept your edited recap — rerun with --day %s --force to replace it", r.Span.Label, r.Span.Label)
		case r.Err != nil:
			failed++
			d.Status, d.Error = lastrun.StatusError, r.Err.Error()
			switch summary.KindOf(r.Err) {
			case summary.KindRefused:
				d.Status = lastrun.StatusRefused
			case summary.KindAuth:
				d.Kind = lastrun.KindAuth
			case summary.KindConfig:
				d.Kind = lastrun.KindConfig
			case summary.KindFatal, summary.KindTransient:
			}
			logErrf("dev day %s: %v", r.Span.Label, r.Err)
		case r.Refused > 0:
			d.Status = lastrun.StatusRefused
			logErrf("dev day %s: recap written, but the model refused to summarize %d project(s)", r.Span.Label, r.Refused)
		case len(warnings) > 0 && r.Summary == nil:
			d.Status = lastrun.StatusPartial
			logf("dev day %s: no sessions found, some sources failed", r.Span.Label)
		case len(warnings) > 0:
			d.Status = lastrun.StatusPartial
			logf("dev day %s: recap written, some sources failed", r.Span.Label)
		case r.Summary == nil:
			d.Status = lastrun.StatusEmpty
			logf("dev day %s: no sessions", r.Span.Label)
		default:
			d.Status = lastrun.StatusOK
			logf("dev day %s: recap written", r.Span.Label)
		}
		days[r.Span.Label] = d
	}
	return failed
}

func stoppedHint(days map[string]lastrun.Day, results []recap.Day, summarizer string) string {
	for _, r := range results {
		if kind := days[r.Span.Label].Kind; kind != "" {
			return fmt.Sprintf("%s — %s to get the recaps going again.", health.Cause(kind, summarizer), health.Fix(kind, summarizer))
		}
	}
	return ""
}

func saveLastRun(results []recap.Day, warnings []string) {
	r := &lastrun.Record{RanAt: time.Now(), Warnings: warnings, Status: lastrun.StatusEmpty}
	for _, d := range results {
		r.DevDay = d.Span.Label
		switch {
		case d.Err != nil:
			r.Status, r.Error = lastrun.StatusError, d.Err.Error()
		case d.Summary != nil && r.Status != lastrun.StatusError:
			r.Status = lastrun.StatusOK
		}
	}
	if r.Status == lastrun.StatusOK && len(warnings) > 0 {
		r.Status = lastrun.StatusPartial
	}
	_ = lastrun.Save(r)
}

func activeSources(cfg *config.Config) ([]parser.Source, error) {
	names := cfg.Sources
	if len(names) == 0 {
		names = []string{parser.SourceClaudeCode}
	}
	var out []parser.Source
	for _, n := range names {
		switch n {
		case parser.SourceClaudeCode:
			cc, err := parser.NewClaudeCode()
			if err != nil {
				return nil, fmt.Errorf("init claude-code: %w", err)
			}
			out = append(out, cc)
		case parser.SourceCodex:
			cx, err := parser.NewCodex()
			if err != nil {
				return nil, fmt.Errorf("init codex: %w", err)
			}
			out = append(out, cx)
		default:
			return nil, fmt.Errorf("unknown source %q in config — run 'jogai init'", n)
		}
	}
	return out, nil
}

var loadClaudeToken = func() (string, error) { return claudetoken.New(claudetoken.Unattended).Load() }

func selectSummarizer(name string) summary.Summarizer {
	if name == summary.NameCodex {
		return summary.Codex{}
	}
	token, err := loadClaudeToken()
	if err != nil {
		logErrf("⚠ %v — using the claude CLI's own login", err)
	}
	return summary.Claude{Token: token}
}
