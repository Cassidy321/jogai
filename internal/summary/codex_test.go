package summary

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cassidy321/jogai/internal/parser"
)

func TestCodexName(t *testing.T) {
	if name := (Codex{}).Name(); name != "codex" {
		t.Errorf("Name = %q, want codex", name)
	}
}

func TestCodexCheckCLI_Missing(t *testing.T) {
	noFallbackDirs(t)
	// Force an empty PATH so the lookup fails regardless of the host.
	t.Setenv("PATH", "")
	err := Codex{}.CheckCLI()
	if err == nil {
		t.Fatal("expected error for missing codex CLI")
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("error should mention codex: %v", err)
	}
}

func TestCodexGenerateNoSessions(t *testing.T) {
	_, err := Codex{}.Generate(context.Background(), Request{Sessions: nil})
	if err == nil {
		t.Error("expected error for empty sessions")
	}
}

func TestCodexGenerate_MissingCLIReportsPath(t *testing.T) {
	noFallbackDirs(t)
	t.Setenv("PATH", "")
	sessions := []parser.Session{{
		ID: "s1", Tool: "codex", Project: "jogai",
		Messages: []parser.Message{{Role: "user", Content: "hi"}},
	}}
	_, err := Codex{}.Generate(context.Background(), Request{Sessions: sessions})
	if err == nil {
		t.Fatal("expected error for missing codex CLI")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error should mention PATH, got: %v", err)
	}
}

func TestCodexGenerate_StubbedBinary(t *testing.T) {
	argsFile := stubCLI(t, "codex", `#!/bin/sh
printf '%s\n' "$@" > "$ARGS_FILE"
while [ $# -gt 0 ]; do
  case "$1" in
    --output-last-message) shift; echo "stub recap body" > "$1"; shift;;
    *) shift;;
  esac
done
exit 0
`)
	sessions := []parser.Session{{
		ID: "s1", Tool: "codex", Project: "jogai",
		Messages: []parser.Message{{Role: "user", Content: "hi"}},
	}}
	s, err := Codex{}.Generate(context.Background(), Request{Sessions: sessions})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s.Content, "stub recap body") {
		t.Errorf("Content = %q, want to contain stub", s.Content)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--ephemeral\n") {
		t.Errorf("codex must run without persisting a session, args = %q", args)
	}
}

func TestCodexGenerate_StubbedBinaryFailure(t *testing.T) {
	dir := t.TempDir()
	stubPath := filepath.Join(dir, "codex")
	script := `#!/bin/sh
echo "boom" >&2
exit 1
`
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	sessions := []parser.Session{{
		ID: "s1", Tool: "codex", Project: "jogai",
		Messages: []parser.Message{{Role: "user", Content: "hi"}},
	}}
	_, err := Codex{}.Generate(context.Background(), Request{Sessions: sessions})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "codex") {
		t.Errorf("error should mention codex: %v", err)
	}
}
