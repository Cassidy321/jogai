package cli

import (
	"errors"
	"fmt"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/summary"
)

type StatusCmd struct{}

func (c *StatusCmd) Run() error {
	fmt.Println("jogai status")
	fmt.Println()

	healthy := true

	cc, err := parser.NewClaudeCode()
	if err != nil {
		fmt.Printf("  Parser:    ✗ error (%v)\n", err)
		healthy = false
	} else if cc.Detect() {
		fmt.Println("  Parser:    ✓ Claude Code")
	} else {
		fmt.Println("  Parser:    ✗ Claude Code not found")
		healthy = false
	}

	if err := summary.CheckCLI(); err != nil {
		fmt.Println("  Summarizer: ✗ claude CLI not found")
		healthy = false
	} else {
		fmt.Println("  Summarizer: ✓ claude CLI")
	}

	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotConfigured) {
			fmt.Println("  Output:    not configured — run 'jogai init'")
			healthy = false
		} else {
			fmt.Printf("  Output:    ✗ error (%v)\n", err)
			healthy = false
		}
	} else {
		fmt.Printf("  Output:    %s\n", cfg.OutputDir)
	}

	lastRun, err := config.LoadLastRun()
	if err != nil {
		if errors.Is(err, config.ErrNeverRun) {
			fmt.Println("  Last run:  never")
		} else {
			fmt.Printf("  Last run:  ✗ error (%v)\n", err)
			healthy = false
		}
	} else {
		fmt.Printf("  Last run:  %s\n", lastRun.Format("Jan 02, 2006 15:04"))
	}

	if !healthy {
		return fmt.Errorf("some checks failed — see above for details")
	}

	return nil
}
