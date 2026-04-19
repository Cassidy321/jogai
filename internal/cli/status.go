package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/scheduler"
	"github.com/Cassidy321/jogai/internal/summary"
)

type StatusCmd struct{}

func (c *StatusCmd) Run() error {
	fmt.Println("jogai status")
	fmt.Println()

	healthy := true

	cc, err := parser.NewClaudeCode()
	if err != nil {
		fmt.Printf("  Parser:     ✗ error (%v)\n", err)
		healthy = false
	} else if cc.Detect() {
		fmt.Println("  Parser:     ✓ Claude Code")
	} else {
		fmt.Println("  Parser:     ✗ Claude Code not found")
		healthy = false
	}

	if err := summary.CheckCLI(); err != nil {
		fmt.Println("  Summarizer: ✗ claude CLI not found")
		healthy = false
	} else {
		fmt.Println("  Summarizer: ✓ claude CLI")
	}

	cfg, err := config.Load()
	switch {
	case errors.Is(err, config.ErrNotConfigured):
		fmt.Println("  Output:     not configured — run 'jogai init'")
		healthy = false
	case err != nil:
		fmt.Printf("  Output:     ✗ error (%v)\n", err)
		healthy = false
	default:
		fmt.Printf("  Output:     %s\n", cfg.OutputDir)
	}

	job, jobErr := loadScheduleJob()
	printScheduleLine(job, jobErr)
	if jobErr != nil || (job != nil && job.Active && job.At == nil) {
		healthy = false
	}
	if cfg != nil && cfg.DayEnd != nil && job != nil && job.Active {
		printStaleRunWarning(cfg, time.Now())
	}

	if !healthy {
		return fmt.Errorf("some checks failed — see above for details")
	}
	return nil
}

func loadScheduleJob() (*scheduler.Job, error) {
	s, err := scheduler.New()
	if err != nil {
		return nil, err
	}
	jobs, err := s.Status()
	if err != nil || len(jobs) == 0 {
		return nil, err
	}
	return &jobs[0], nil
}

func printScheduleLine(job *scheduler.Job, err error) {
	switch {
	case err != nil:
		fmt.Printf("  Schedule:   ✗ error (%v)\n", err)
	case job == nil:
		fmt.Println("  Schedule:   unknown")
	case !job.Active:
		fmt.Println("  Schedule:   none (run `jogai schedule start` to enable)")
	case job.At == nil:
		fmt.Println("  Schedule:   active but dev day boundary not configured — run `jogai init`")
	default:
		fmt.Printf("  Schedule:   daily at %s, next run %s\n",
			job.At, job.NextRun.Format("2006-01-02 15:04"))
	}
}

func printStaleRunWarning(cfg *config.Config, now time.Time) {
	if cfg.OutputDir == "" || cfg.DayEnd == nil {
		return
	}
	_, _, label := devday.Previous(now, *cfg.DayEnd)
	expected := filepath.Join(cfg.OutputDir, label+".md")
	if _, err := os.Stat(expected); err == nil {
		return
	}
	fmt.Printf("\n  ! Last scheduled run didn't produce %s.\n", label+".md")
	fmt.Printf("    Catch up with: jogai run --day %s\n", label)
}
