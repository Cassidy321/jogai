package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/filter"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/summary"
)

type RunCmd struct {
	Period string `help:"Recap period: session, daily, weekly, monthly." default:"daily" enum:"session,daily,weekly,monthly"`
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

	since, err := config.LoadLastRunFor(c.Period)
	if err != nil {
		since = time.Now().Add(-24 * time.Hour)
		if err != config.ErrNeverRun {
			fmt.Printf("Warning: %v — falling back to last 24h\n", err)
		}
	}
	fmt.Printf("Parsing sessions since %s...\n", since.Format("Jan 02 15:04"))

	sessions, err := cc.Sessions(since)
	if err != nil {
		return fmt.Errorf("parse sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No new sessions found.")
		return nil
	}

	fmt.Printf("Found %d session(s). Filtering...\n", len(sessions))
	filtered := filter.Reduce(sessions)

	fmt.Println("Generating summary...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s, err := summary.Generate(ctx, filtered, c.Period)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	md := output.NewMarkdown(cfg.OutputDir)
	if err := md.Write(s); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	if err := config.SaveLastRunFor(c.Period, time.Now()); err != nil {
		return fmt.Errorf("save last run: %w", err)
	}

	fmt.Printf("Done! Recap written to %s\n", cfg.OutputDir)
	return nil
}
