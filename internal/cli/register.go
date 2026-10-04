package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/scheduler"
	"github.com/Cassidy321/jogai/internal/summary"
	"github.com/Cassidy321/jogai/internal/update"
)

const mcpName = "jogai"

// The marker avoids `claude mcp get` on every run (it starts the server to
// probe it), and leaves alone a registration the user removed on purpose.
func registerMCP(exe string) (bool, error) {
	if exe == "" {
		return false, nil
	}
	dir, err := config.Dir()
	if err != nil {
		return false, err
	}
	marker := filepath.Join(dir, "mcp_registered")
	if data, err := os.ReadFile(marker); err == nil && string(data) == exe {
		return false, nil
	}
	claude, err := summary.LookPath(summary.NameClaude)
	if errors.Is(err, summary.ErrCLINotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	run := func(args ...string) ([]byte, error) {
		cmd := exec.Command(claude, args...)
		cmd.Dir = os.TempDir()
		return cmd.CombinedOutput()
	}
	_, _ = run("mcp", "remove", "--scope", "user", mcpName)
	if out, err := run("mcp", "add", "--scope", "user", mcpName, "--", exe, "mcp"); err != nil {
		return false, fmt.Errorf("claude mcp add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return true, err
	}
	return true, os.WriteFile(marker, []byte(exe), 0o644)
}

// A Homebrew upgrade deletes the versioned Cellar path: register the stable
// <prefix>/bin symlink instead.
func stableExecutable() string {
	exe, err := os.Executable()
	if err != nil || scheduler.IsTempBinary(exe) {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		if prefix, ok := update.HomebrewPrefix(resolved); ok {
			return filepath.Join(prefix, "bin", "jogai")
		}
	}
	return exe
}
