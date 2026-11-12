package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Cassidy321/jogai/internal/claudetoken"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/parser"
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
	if !printSourcesSection(cfg, det, derr) {
		healthy = false
	}

	if !printSummarizerSection(cfg) {
		healthy = false
	}

	if !printOutputSection(cfg, cfgErr) {
		healthy = false
	}
	if !printArchiveLine() {
		healthy = false
	}

	job, jobErr := loadScheduleJob()
	printScheduleLine(job, jobErr)
	if jobErr != nil || (job != nil && job.Active && job.At == nil) {
		healthy = false
	}

	printLastRun()

	if cfg != nil && cfg.DayEnd != nil && !printDayIssues(cfg, time.Now()) {
		healthy = false
	}

	if !healthy {
		return fmt.Errorf("some checks failed — see above for details")
	}
	return nil
}

func printSourcesSection(cfg *config.Config, det detectedSources, derr error) bool {
	if derr != nil {
		fmt.Printf("  Sources:    ✗ error (%v)\n", derr)
		return false
	}
	healthy := true
	printSourcesStatus(det, cfg)
	if cfg != nil && cfg.Sources != nil {
		for _, name := range cfg.Sources {
			if (name == parser.SourceClaudeCode && !det.claudeCode) || (name == parser.SourceCodex && !det.codex) {
				healthy = false
			}
		}
	}
	if det.codex && (cfg == nil || !slices.Contains(cfg.Sources, parser.SourceCodex)) {
		fmt.Println("  ℹ Codex detected but not enabled — run 'jogai init' to add it")
	}
	return healthy
}

func printSummarizerSection(cfg *config.Config) bool {
	summarizer := summarizerName(cfg)
	var sizer summary.Summarizer
	switch summarizer {
	case summary.NameCodex:
		sizer = summary.Codex{}
	default:
		sizer = summary.Claude{}
	}
	if err := sizer.CheckCLI(); err != nil {
		fmt.Printf("  Summarizer: ✗ %s CLI not found\n", summarizer)
		return false
	}
	fmt.Printf("  Summarizer: ✓ %s CLI%s\n", summarizer, tokenNote(summarizer, time.Now()))
	return true
}

func tokenNote(summarizer string, now time.Time) string {
	st, ok := claudetoken.LoadState()
	if !ok || summarizer != summary.NameClaude {
		return ""
	}
	date := st.ExpiresAt().Format("2006-01-02")
	if st.Expired(now) {
		return fmt.Sprintf(" (token expired on %s — run `jogai login`)", date)
	}
	return fmt.Sprintf(" (token valid until %s)", date)
}

func printOutputSection(cfg *config.Config, cfgErr error) bool {
	switch {
	case errors.Is(cfgErr, config.ErrNotConfigured):
		fmt.Println("  Output:     not configured — run 'jogai init'")
		return false
	case cfgErr != nil:
		fmt.Printf("  Output:     ✗ error (%v)\n", cfgErr)
		return false
	default:
		fmt.Printf("  Output:     %s\n", cfg.OutputDir)
		return true
	}
}

func printSourcesStatus(det detectedSources, cfg *config.Config) {
	active := map[string]bool{}
	if cfg != nil {
		for _, s := range cfg.Sources {
			active[s] = true
		}
	}
	if cfg == nil || cfg.Sources == nil {
		active[parser.SourceClaudeCode] = true
	}

	if det.claudeCode {
		mark := "✓"
		if !active[parser.SourceClaudeCode] {
			mark = "·"
		}
		fmt.Printf("  Sources:    %s Claude Code\n", mark)
	} else if active[parser.SourceClaudeCode] {
		fmt.Println("  Sources:    ✗ Claude Code (not installed, still in config)")
	}

	if det.codex {
		mark := "✓"
		if !active[parser.SourceCodex] {
			mark = "·"
		}
		fmt.Printf("              %s Codex\n", mark)
	} else if active[parser.SourceCodex] {
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
		fmt.Println(hanging("              ✗ ", r.Error))
	}
}

func loadScheduleJob() (*scheduler.Job, error) {
	s, err := scheduler.New()
	if err != nil {
		return nil, err
	}
	job, err := s.Status()
	if err != nil {
		return nil, err
	}
	return &job, nil
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

func printDayIssues(cfg *config.Config, now time.Time) bool {
	days, err := lastrun.LoadDays()
	if err != nil {
		fmt.Printf("  History:    ✗ %v\n", err)
		return false
	}
	exists := recapExists(cfg.OutputDir)
	ok := true
	pending := 0
	for _, s := range devday.Recent(now, *cfg.DayEnd, catchUpDays) {
		d, recorded := days[s.Label]
		switch {
		case recorded && d.Status == lastrun.StatusError:
			ok = false
			fmt.Println(dayIssue(s.Label, d.Error))
		case recorded && d.Status == lastrun.StatusRefused:
			ok = false
			fmt.Printf("  ! %s was refused by the model — retry with: jogai run --day %s\n", s.Label, s.Label)
		case !recorded && !exists(s.Label):
			pending++
		}
	}
	if pending > 0 {
		fmt.Printf("  ℹ %d dev day(s) not recapped yet — the next run catches up (or run `jogai run` now)\n", pending)
	}
	return ok
}

func dayIssue(label, failure string) string {
	head := fmt.Sprintf("  ! %s failed, retried on the next run:", label)
	lines := nonBlankLines(failure)
	if len(lines) <= 1 {
		return head + " " + strings.Join(lines, "")
	}
	return head + "\n      " + strings.Join(lines, "\n      ")
}

func hanging(prefix, text string) string {
	indent := strings.Repeat(" ", utf8.RuneCountInString(prefix))
	return prefix + strings.Join(nonBlankLines(text), "\n"+indent)
}

func nonBlankLines(text string) []string {
	var out []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
