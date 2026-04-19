package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/recap"
	"github.com/Cassidy321/jogai/internal/summary"
)

type RunCmd struct {
	Day string `name:"day" help:"Recap a specific dev day (YYYY-MM-DD)."`

	// Legacy flags from v0.4 plists. Accepted silently during the v0.5 transition
	// so pre-existing launchd jobs keep working until they're regenerated.
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

	cc, err := parser.NewClaudeCode()
	if err != nil {
		return fmt.Errorf("init parser: %w", err)
	}

	if !cc.Detect() {
		return fmt.Errorf("claude Code not found — no sessions to parse")
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

	p := &recap.Pipeline{
		Parser:     cc,
		Summarizer: recap.SummarizerFunc(summary.Generate),
		Writer:     output.NewMarkdown(cfg.OutputDir),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s, err := p.Run(ctx, since, until, since)
	if err != nil {
		return err
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
