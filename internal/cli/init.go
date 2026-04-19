package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/parser"
)

type InitCmd struct{}

func (c *InitCmd) Run() error {
	fmt.Println("jogai init — setting up your AI session recaps")
	fmt.Println()

	cc, err := parser.NewClaudeCode()
	if err != nil {
		return err
	}

	if cc.Detect() {
		fmt.Println("  ✓ Claude Code detected")
	} else {
		fmt.Println("  ✗ Claude Code not found")
		return fmt.Errorf("no supported AI tool found — install Claude Code first")
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

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Where should recaps be saved?").
				Description("Markdown files will be written here (works great with Obsidian or any notes folder)").
				Value(&outputDir),
			huh.NewInput().
				Title("What time does your dev day end? (HH:MM)").
				Description("Leave 00:00 for calendar days, or pick a morning hour to capture late-night sessions (e.g. 05:00)").
				Value(&dayEnd).
				Validate(validateTimeOfDay),
		),
	).WithTheme(jogaiTheme())
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
		OutputDir: outputDir,
		DayEnd:    &parsedDayEnd,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("\n  ✓ Config saved\n")
	fmt.Printf("  ✓ Recaps will be written to %s\n", outputDir)
	fmt.Printf("  ✓ Dev day ends at %s\n", parsedDayEnd)

	if err := probeWriteAccess(outputDir); err != nil {
		fmt.Printf("\n  ! Could not write to %s: %s\n", outputDir, err)
		fmt.Println("    If macOS showed a permission prompt, accept it.")
		fmt.Println("    Otherwise: System Settings → Privacy & Security → Files and Folders → grant access to jogai.")
		return nil
	}

	fmt.Println("\nRun 'jogai run' to generate your first recap.")
	return nil
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
