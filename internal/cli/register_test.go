package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cassidy321/jogai/internal/summary"
)

func TestRegisterMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := t.TempDir()
	args := filepath.Join(bin, "args")
	stub := "#!/bin/sh\necho \"$@\" >> " + args + "\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	changed, err := registerMCP("/opt/homebrew/bin/jogai")
	if err != nil || !changed {
		t.Fatalf("first registration = (%v, %v)", changed, err)
	}
	data, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "mcp add --scope user jogai -- /opt/homebrew/bin/jogai mcp") {
		t.Errorf("claude was called with:\n%s", data)
	}
	if changed, err := registerMCP("/opt/homebrew/bin/jogai"); err != nil || changed {
		t.Errorf("second registration = (%v, %v), want nothing to do", changed, err)
	}
	if changed, err := registerMCP(""); err != nil || changed {
		t.Errorf("a temporary build must not register, got (%v, %v)", changed, err)
	}
}

func TestRegisterMCP_WithoutClaudeCLIIsANoOp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	if _, err := summary.LookPath(summary.NameClaude); err == nil {
		t.Skip("claude is installed in a system-wide location on this machine")
	}
	if changed, err := registerMCP("/opt/homebrew/bin/jogai"); err != nil || changed {
		t.Errorf("registerMCP = (%v, %v), want a silent no-op for codex-only setups", changed, err)
	}
}
