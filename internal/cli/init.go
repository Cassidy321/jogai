package cli

import (
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
	defaultDir := filepath.Join(home, "jogai-recaps")

	var outputDir string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Where should recaps be saved?").
				Description("Markdown files will be written here (works with Obsidian)").
				Value(&outputDir).
				Placeholder(defaultDir),
		),
	).Run()
	if err != nil {
		return err
	}

	if outputDir == "" {
		outputDir = defaultDir
	}

	if len(outputDir) >= 2 && outputDir[:2] == "~/" {
		outputDir = filepath.Join(home, outputDir[2:])
	}

	cfg := &config.Config{
		OutputDir: outputDir,
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("\n  ✓ Config saved\n")
	fmt.Printf("  ✓ Recaps will be written to %s\n\n", outputDir)
	fmt.Println("Run 'jogai run' to generate your first recap.")

	return nil
}
