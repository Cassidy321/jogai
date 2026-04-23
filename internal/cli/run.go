package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/recap"
	"github.com/Cassidy321/jogai/internal/summary"
)

type RunCmd struct {
	Day string `name:"day" help:"Recap a specific dev day (YYYY-MM-DD)."`

	// Legacy v0.4 flags, kept hidden for schedule backward compatibility.
	Scheduled bool   `kong:"hidden"`
	At        string `kong:"hidden"`
}

func (c *RunCmd) Run() error {
	release, err := config.AcquireLock()
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

	now := time.Now()
	since, until, err := c.window(now, *cfg.DayEnd)
	if err != nil {
		return err
	}
	fmt.Printf("Recapping dev day %s (%s → %s)\n",
		since.Format(devday.LabelFormat),
		since.Format("Jan 02 15:04"),
		until.Format("Jan 02 15:04"),
	)

	multi := &parser.MultiParser{Parsers: parsers}
	p := &recap.Pipeline{
		Parser:     multi,
		Summarizer: sizer,
		Writer:     output.NewMarkdown(cfg.OutputDir),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	devDayLabel := since.Format(devday.LabelFormat)
	s, runErr := p.Run(ctx, since, until, since)

	for _, w := range multi.Warnings() {
		fmt.Printf("⚠ %s\n", w)
	}

	writeLastRun(runErr, multi.Warnings(), devDayLabel, s)

	if runErr != nil {
		return runErr
	}
	if s == nil {
		fmt.Println("No new sessions found.")
		return nil
	}
	fmt.Printf("Done! Recap written to %s\n", cfg.OutputDir)
	return nil
}

func (c *RunCmd) window(now time.Time, dayEnd config.TimeOfDay) (since, until time.Time, err error) {
	if c.Day == "" {
		since, until, _ = devday.Previous(now, dayEnd)
		return since, until, nil
	}
	date, err := time.ParseInLocation(devday.LabelFormat, c.Day, now.Location())
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --day date %q — expected YYYY-MM-DD", c.Day)
	}
	since, until, _ = devday.FromDate(date, dayEnd)
	if until.After(now) {
		return time.Time{}, time.Time{}, fmt.Errorf(
			"dev day %s is not yet complete — window ends at %s",
			c.Day,
			until.Format("2006-01-02 15:04"),
		)
	}
	return since, until, nil
}

func buildActiveParsers(cfg *config.Config) ([]parser.Parser, error) {
	names := cfg.Sources
	if len(names) == 0 {
		names = []string{"claude-code"}
	}
	var out []parser.Parser
	for _, n := range names {
		switch n {
		case "claude-code":
			cc, err := parser.NewClaudeCode()
			if err != nil {
				return nil, fmt.Errorf("init claude-code: %w", err)
			}
			out = append(out, cc)
		case "codex":
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
	case "codex":
		return summary.Codex{}
	default:
		return summary.Claude{}
	}
}

func writeLastRun(runErr error, warnings []string, devDay string, s *summary.Summary) {
	r := &lastrun.Record{
		RanAt:    time.Now(),
		DevDay:   devDay,
		Warnings: warnings,
	}
	switch {
	case runErr != nil:
		r.Status = "error"
		r.Error = runErr.Error()
	case len(warnings) > 0 && s != nil:
		r.Status = "partial"
	case s == nil:
		r.Status = "empty"
	default:
		r.Status = "ok"
	}
	_ = lastrun.Save(r)
}
