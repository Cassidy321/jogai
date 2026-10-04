package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/scheduler"
	"github.com/Cassidy321/jogai/internal/summary"
)

type UninstallCmd struct {
	Data bool `help:"Also delete the session archive and jogai's settings."`
}

func (c *UninstallCmd) Run() error {
	if s, err := scheduler.New(); err == nil {
		if err := s.Uninstall(); err != nil {
			return err
		}
		fmt.Println("  ✓ schedule removed")
	}
	if claude, err := summary.LookPath(summary.NameClaude); err == nil {
		cmd := exec.Command(claude, "mcp", "remove", "--scope", "user", mcpName)
		cmd.Dir = os.TempDir()
		if cmd.Run() == nil {
			fmt.Println("  ✓ removed from Claude Code")
		}
	}
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(dir, "mcp_registered"))
	if c.Data {
		archivePath, err := archive.DefaultPath()
		if err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Dir(archivePath)); err != nil {
			return err
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
		fmt.Println("  ✓ archive and settings deleted")
	}
	fmt.Println("\nTo remove the binary: brew uninstall jogai")
	return nil
}
