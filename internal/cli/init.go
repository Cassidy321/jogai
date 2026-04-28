package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/parser"
)

type InitCmd struct{}

type detectedSources struct {
	claudeCode bool
	codex      bool
}

func (d detectedSources) names() []string {
	var out []string
	if d.claudeCode {
		out = append(out, "claude-code")
	}
	if d.codex {
		out = append(out, "codex")
	}
	return out
}

type detectedSummarizers struct {
	claude bool
	codex  bool
}

func resolveSummarizer(det detectedSummarizers, existing string) string {
	switch {
	case det.claude && det.codex:
		if existing == "claude" || existing == "codex" {
			return existing
		}
		return "" // caller will prompt
	case det.claude:
		return "claude"
	case det.codex:
		return "codex"
	default:
		return ""
	}
}

func (c *InitCmd) Run() error {
	fmt.Println("jogai init — setting up your AI session recaps")
	fmt.Println()

	det, err := detectSources()
	if err != nil {
		return err
	}
	if !det.claudeCode && !det.codex {
		return fmt.Errorf("no supported AI tool found — install Claude Code or Codex first")
	}
	printDetectedSources(det)

	detSum := detectSummarizers()
	if !detSum.claude && !detSum.codex {
		return fmt.Errorf("no summarizer CLI found — install the `claude` or `codex` CLI")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	existing, _ := config.Load()
	if existing != nil {
		fmt.Println("  ✓ Existing config found — press Enter to keep current values")
	}

	outputDir := defaultOutputDir(existing, filepath.Join(home, "jogai-recaps"))
	dayEnd := defaultDayEnd(existing)

	selectedSources := defaultSelectedSources(existing, det)
	summarizer := resolveSummarizer(detSum, defaultSummarizer(existing))

	form := buildInitForm(det, &outputDir, &dayEnd, &selectedSources, &summarizer)
	if err := form.Run(); err != nil {
		return err
	}

	outputDir = expandHome(outputDir, home)
	parsedDayEnd, err := config.ParseTimeOfDay(dayEnd)
	if err != nil {
		return fmt.Errorf("invalid day_end: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("cannot create output directory %s: %w", outputDir, err)
	}

	cfg := &config.Config{
		OutputDir:  outputDir,
		DayEnd:     &parsedDayEnd,
		Sources:    selectedSources,
		Summarizer: summarizer,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("\n  ✓ Config saved\n")
	fmt.Printf("  ✓ Recaps will be written to %s\n", outputDir)
	fmt.Printf("  ✓ Dev day ends at %s\n", parsedDayEnd)
	fmt.Printf("  ✓ Sources: %v\n", selectedSources)
	fmt.Printf("  ✓ Summarizer: %s\n", summarizer)

	if err := probeWriteAccess(outputDir); err != nil {
		fmt.Printf("\n  ! Could not write to %s: %s\n", outputDir, err)
		fmt.Println("    If macOS showed a permission prompt, accept it.")
		fmt.Println("    Otherwise: System Settings → Privacy & Security → Files and Folders → grant access to jogai.")
		return nil
	}

	fmt.Println("\nRun 'jogai run' to generate your first recap.")
	return nil
}

func buildInitForm(det detectedSources, outputDir, dayEnd *string, selectedSources *[]string, summarizer *string) *huh.Form {
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Title("Where should recaps be saved?").
				Description("Markdown files will be written here (works great with Obsidian or any notes folder)").
				Value(outputDir),
			huh.NewInput().
				Title("What time does your dev day end? (HH:MM)").
				Description("Leave 00:00 for calendar days, or pick a morning hour to capture late-night sessions (e.g. 05:00)").
				Value(dayEnd).
				Validate(validateTimeOfDay),
		),
	}

	if len(det.names()) > 1 {
		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which AI tools should jogai recap?").
				Options(sourceOptions(det)...).
				Value(selectedSources).
				Validate(func(v []string) error {
					if len(v) == 0 {
						return fmt.Errorf("select at least one source")
					}
					return nil
				}),
		))
	}

	if *summarizer == "" {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which CLI should generate the recap?").
				Description("Both Claude Code and Codex CLIs are installed — pick the one you prefer for summaries").
				Options(
					huh.NewOption("Claude", "claude"),
					huh.NewOption("Codex", "codex"),
				).
				Value(summarizer),
		))
	}

	return huh.NewForm(groups...).WithTheme(jogaiTheme())
}

func detectSources() (detectedSources, error) {
	cc, err := parser.NewClaudeCode()
	if err != nil {
		return detectedSources{}, err
	}
	cx, err := parser.NewCodex()
	if err != nil {
		return detectedSources{}, err
	}
	return detectedSources{claudeCode: cc.Detect(), codex: cx.Detect()}, nil
}

func printDetectedSources(d detectedSources) {
	if d.claudeCode {
		fmt.Println("  ✓ Claude Code detected")
	} else {
		fmt.Println("  ✗ Claude Code not found")
	}
	if d.codex {
		fmt.Println("  ✓ Codex detected")
	} else {
		fmt.Println("  ✗ Codex not found")
	}
}

func detectSummarizers() detectedSummarizers {
	_, errC := exec.LookPath("claude")
	_, errX := exec.LookPath("codex")
	return detectedSummarizers{claude: errC == nil, codex: errX == nil}
}

func sourceOptions(d detectedSources) []huh.Option[string] {
	var opts []huh.Option[string]
	if d.claudeCode {
		opts = append(opts, huh.NewOption("Claude Code", "claude-code"))
	}
	if d.codex {
		opts = append(opts, huh.NewOption("Codex", "codex"))
	}
	return opts
}

func defaultSelectedSources(existing *config.Config, det detectedSources) []string {
	if existing != nil && len(existing.Sources) > 0 {
		return existing.Sources
	}
	return det.names()
}

func defaultSummarizer(existing *config.Config) string {
	if existing != nil {
		return existing.Summarizer
	}
	return ""
}

func defaultOutputDir(existing *config.Config, fallback string) string {
	if existing != nil && existing.OutputDir != "" {
		return existing.OutputDir
	}
	return fallback
}

func defaultDayEnd(existing *config.Config) string {
	if existing != nil && existing.DayEnd != nil {
		return existing.DayEnd.String()
	}
	return "00:00"
}

func validateTimeOfDay(s string) error {
	_, err := config.ParseTimeOfDay(s)
	return err
}

func expandHome(path, home string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(home, path[2:])
	}
	return path
}

// probeWriteAccess writes and removes a sentinel file in dir to surface macOS
// TCC prompts during init, while the user is present to grant access, rather
// than at 05:00 AM when the schedule first fires.
func probeWriteAccess(dir string) error {
	path := filepath.Join(dir, ".jogai-write-test")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_ = f.Close()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
