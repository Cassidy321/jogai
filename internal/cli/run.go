package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/recap"
	"github.com/Cassidy321/jogai/internal/scheduler"
	"github.com/Cassidy321/jogai/internal/summary"
)

type RunCmd struct {
	Day       string `name:"day" aliases:"since" help:"Recap a specific day (YYYY-MM-DD)."`
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

	cc, err := parser.NewClaudeCode()
	if err != nil {
		return fmt.Errorf("init parser: %w", err)
	}

	if !cc.Detect() {
		return fmt.Errorf("claude Code not found — no sessions to parse")
	}

	now := time.Now()
	since, until, recapDate, err := c.timeWindow(now)
	if err != nil {
		return err
	}
	fmt.Printf("Parsing sessions from %s to %s...\n", since.Format("Jan 02 15:04"), until.Format("Jan 02 15:04"))

	p := &recap.Pipeline{
		Parser:     cc,
		Summarizer: recap.SummarizerFunc(summary.Generate),
		Writer:     output.NewMarkdown(cfg.OutputDir),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s, err := p.Run(ctx, since, until, recapDate)
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

func (c *RunCmd) timeWindow(now time.Time) (since, until, recapDate time.Time, err error) {
	if c.Day != "" && c.Scheduled {
		return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("--day cannot be combined with --scheduled")
	}

	if c.Day != "" {
		day, parseErr := time.ParseInLocation("2006-01-02", c.Day, now.Location())
		if parseErr != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("invalid --day date %q — expected YYYY-MM-DD", c.Day)
		}
		return day, day.AddDate(0, 0, 1), day, nil
	}

	if c.Scheduled {
		if c.At == "" {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("scheduled runs require --at HH:MM")
		}
		sched, parseErr := scheduler.ParseAt(c.At)
		if parseErr != nil {
			return time.Time{}, time.Time{}, time.Time{}, parseErr
		}
		since, until = scheduler.Window(sched, now)
		return since, until, until, nil
	}

	return now.Add(-24 * time.Hour), now, now, nil
}
