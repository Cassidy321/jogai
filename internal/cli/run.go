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

	allSessions, err := cc.Sessions(since)
	if err != nil {
		return fmt.Errorf("parse sessions: %w", err)
	}

	// Filter sessions within the time window.
	var sessions []parser.Session
	for _, s := range allSessions {
		if s.StartedAt.Before(until) {
			sessions = append(sessions, s)
		}
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

	s, err := summary.Generate(ctx, filtered)
	if err != nil {
		return fmt.Errorf("generate summary: %w", err)
	}

	md := output.NewMarkdown(cfg.OutputDir)
	if err := md.Write(s); err != nil {
		return fmt.Errorf("write output: %w", err)
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
