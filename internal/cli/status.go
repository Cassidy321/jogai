package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/scheduler"
	"github.com/Cassidy321/jogai/internal/summary"
)

type StatusCmd struct{}

func (c *StatusCmd) Run() error {
	fmt.Println("jogai status")
	fmt.Println()

	cfg, cfgErr := config.Load()
	healthy := true

	det, derr := detectSources()
	if derr != nil {
		fmt.Printf("  Sources:    ✗ error (%v)\n", derr)
		healthy = false
	} else {
		printSourcesStatus(det, cfg)
		if cfg != nil && cfg.Sources != nil {
			for _, name := range cfg.Sources {
				if (name == "claude-code" && !det.claudeCode) || (name == "codex" && !det.codex) {
					healthy = false
				}
			}
		}
	}

	if det.codex && (cfg == nil || !containsString(cfg.Sources, "codex")) {
		fmt.Println("  ℹ Codex detected but not enabled — run 'jogai init' to add it")
	}

	summarizer := "claude"
	if cfg != nil && cfg.Summarizer != "" {
		summarizer = cfg.Summarizer
	}
	var sizer summary.Summarizer
	switch summarizer {
	case "codex":
		sizer = summary.Codex{}
	default:
		sizer = summary.Claude{}
	}
	if err := sizer.CheckCLI(); err != nil {
		fmt.Printf("  Summarizer: ✗ %s CLI not found\n", summarizer)
		healthy = false
	} else {
		fmt.Printf("  Summarizer: ✓ %s CLI\n", summarizer)
	}

	switch {
	case errors.Is(cfgErr, config.ErrNotConfigured):
		fmt.Println("  Output:     not configured — run 'jogai init'")
		healthy = false
	case cfgErr != nil:
		fmt.Printf("  Output:     ✗ error (%v)\n", cfgErr)
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

	printLastRun()

	if !healthy {
		return fmt.Errorf("some checks failed — see above for details")
	}
	return nil
}

func printSourcesStatus(det detectedSources, cfg *config.Config) {
	active := map[string]bool{}
	if cfg != nil {
		for _, s := range cfg.Sources {
			active[s] = true
		}
	}
	if cfg == nil || cfg.Sources == nil {
		active["claude-code"] = true
	}

	if det.claudeCode {
		mark := "✓"
		if !active["claude-code"] {
			mark = "·"
		}
		fmt.Printf("  Sources:    %s Claude Code\n", mark)
	} else if active["claude-code"] {
		fmt.Println("  Sources:    ✗ Claude Code (not installed, still in config)")
	}

	if det.codex {
		mark := "✓"
		if !active["codex"] {
			mark = "·"
		}
		fmt.Printf("              %s Codex\n", mark)
	} else if active["codex"] {
		fmt.Println("              ✗ Codex (not installed, still in config)")
	}
}

func printLastRun() {
	r, err := lastrun.Load()
	if err != nil {
		fmt.Printf("  Last run:   ✗ error (%v)\n", err)
		return
	}
	if r == nil {
		return
	}
	fmt.Printf("  Last run:   %s (dev day %s) — %s\n",
		r.RanAt.Local().Format("2006-01-02 15:04"),
		r.DevDay,
		r.Status,
	)
	for _, w := range r.Warnings {
		fmt.Printf("              ⚠ %s\n", w)
	}
	if r.Error != "" {
		fmt.Printf("              ✗ %s\n", r.Error)
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
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
