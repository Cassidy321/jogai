package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/recap"
	"github.com/Cassidy321/jogai/internal/summary"
)

const catchUpDays = 14

type RunCmd struct {
	Day string `name:"day" help:"Recap a specific dev day (YYYY-MM-DD), even if it was already recapped."`

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

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DayEnd == nil {
		return fmt.Errorf("dev day boundary not configured — run 'jogai init' to set it")
	}
	return c.recapPending(cfg)
}

func (c *RunCmd) recapPending(cfg *config.Config) error {
	parsers, err := buildActiveParsers(cfg)
	if err != nil {
		return err
	}
	if len(parsers) == 0 {
		return fmt.Errorf("no sources configured — run 'jogai init'")
	}
	sizer := selectSummarizer(cfg.Summarizer)
	if err := sizer.CheckCLI(); err != nil {
		return err
	}

	days := loadDays()
	spans, err := c.spans(time.Now(), *cfg.DayEnd, days, recapExists(cfg.OutputDir))
	if err != nil {
		return err
	}
	if len(spans) == 0 {
		logf("Every recent dev day is already recapped.")
		return nil
	}
	logf("Recapping %s", describeSpans(spans))

	multi := &parser.MultiParser{Parsers: parsers}
	p := &recap.Pipeline{
		Parser:     multi,
		Summarizer: summary.NewRetry(sizer),
		Writer:     output.NewMarkdown(cfg.OutputDir),
	}
	results, err := p.Run(context.Background(), spans)
	if err != nil {
		results = failAll(spans, err)
	}
	warnings := multi.Warnings()
	for _, w := range warnings {
		logf("⚠ %s", w)
	}

	failed := record(days, results, warnings)
	if err := lastrun.SaveDays(days); err != nil {
		logErrf("⚠ could not save the run history: %v", err)
	}
	saveLastRun(results, warnings)
	if failed > 0 {
		return fmt.Errorf("%d dev day(s) failed — see the errors above", failed)
	}
	return nil
}

func (c *RunCmd) spans(now time.Time, dayEnd config.TimeOfDay, days map[string]lastrun.Day, exists func(label string) bool) ([]devday.Span, error) {
	if c.Day == "" {
		return pendingSpans(devday.Recent(now, dayEnd, catchUpDays), days, exists), nil
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
func pendingSpans(candidates []devday.Span, days map[string]lastrun.Day, exists func(label string) bool) []devday.Span {
	var out []devday.Span
	for _, s := range candidates {
		d, recorded := days[s.Label]
		if recorded && !d.NeedsRetry() {
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

func failAll(spans []devday.Span, err error) []recap.Day {
	days := make([]recap.Day, len(spans))
	for i, s := range spans {
		days[i] = recap.Day{Span: s, Err: err}
	}
	return days
}

func record(days map[string]lastrun.Day, results []recap.Day, warnings []string) int {
	failed := 0
	now := time.Now()
	for _, r := range results {
		d := lastrun.Day{UpdatedAt: now}
		switch {
		case r.Err != nil:
			failed++
			d.Status, d.Error = lastrun.StatusError, r.Err.Error()
			if summary.KindOf(r.Err) == summary.KindRefused {
				d.Status = lastrun.StatusRefused
			}
			logErrf("dev day %s: %v", r.Span.Label, r.Err)
		case r.Summary == nil:
			d.Status = lastrun.StatusEmpty
			logf("dev day %s: no sessions", r.Span.Label)
		case len(warnings) > 0:
			d.Status = lastrun.StatusPartial
			logf("dev day %s: recap written, some sources failed", r.Span.Label)
		default:
			d.Status = lastrun.StatusOK
			logf("dev day %s: recap written", r.Span.Label)
		}
		days[r.Span.Label] = d
	}
	return failed
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

func buildActiveParsers(cfg *config.Config) ([]parser.Parser, error) {
	names := cfg.Sources
	if len(names) == 0 {
		names = []string{parser.SourceClaudeCode}
	}
	var out []parser.Parser
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

func selectSummarizer(name string) summary.Summarizer {
	switch name {
	case summary.NameCodex:
		return summary.Codex{}
	default:
		return summary.Claude{}
	}
}
