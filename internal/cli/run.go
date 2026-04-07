package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/recap"
	"github.com/Cassidy321/jogai/internal/summary"
)

type RunCmd struct {
	Since string `help:"Recap a specific day (YYYY-MM-DD). Defaults to last 24h." default:""`
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
		return fmt.Errorf("Claude Code not found — no sessions to parse")
	}

	since, until, err := c.timeWindow()
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

	s, err := p.Run(ctx, since, until)
	if err != nil {
		return err
	}

	if s == nil {
		fmt.Println("No new sessions found.")
		return nil
	}

	if c.Since == "" {
		if err := config.SaveLastRun(time.Now()); err != nil {
			return fmt.Errorf("save last run: %w", err)
		}
	}

	fmt.Printf("Done! Recap written to %s\n", cfg.OutputDir)
	return nil
}

func (c *RunCmd) timeWindow() (since, until time.Time, err error) {
	if c.Since != "" {
		day, parseErr := time.Parse("2006-01-02", c.Since)
		if parseErr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --since date %q — expected YYYY-MM-DD", c.Since)
		}
		return day, day.AddDate(0, 0, 1), nil
	}

	since, err = config.LoadLastRun()
	if err != nil {
		since = time.Now().Add(-24 * time.Hour)
		if err != config.ErrNeverRun {
			fmt.Printf("Warning: %v — falling back to last 24h\n", err)
		}
	}
	return since, time.Now(), nil
}
