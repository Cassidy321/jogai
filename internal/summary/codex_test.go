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

func TestCodexGenerate_IgnoresTheUserConfig(t *testing.T) {
	argsFile := stubCLI(t, "codex", `#!/bin/sh
printf '%s\n' "$@" > "$ARGS_FILE"
while [ $# -gt 0 ]; do
  case "$1" in
    --output-last-message) shift; echo "stub recap body" > "$1"; shift;;
    *) shift;;
  esac
done
`)
	if _, err := (Codex{}).Generate(context.Background(), Request{Sessions: oneSession}); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "--ignore-user-config\n") {
		t.Errorf("codex must not read the user's config.toml, args = %q", args)
	}
}

func TestCodexGenerate_OlderCodexRunsWithTheUserConfig(t *testing.T) {
	argsFile := stubCLI(t, "codex", `#!/bin/sh
echo call >> "$ARGS_FILE.calls"
for arg in "$@"; do
  if [ "$arg" = "--ignore-user-config" ]; then
    echo "error: unexpected argument '--ignore-user-config' found" >&2
    echo >&2
    echo "Usage: codex exec [OPTIONS] [PROMPT]" >&2
    exit 2
  fi
done
printf '%s\n' "$@" > "$ARGS_FILE"
while [ $# -gt 0 ]; do
  case "$1" in
    --output-last-message) shift; echo "stub recap body" > "$1"; shift;;
    *) shift;;
  esac
done
`)
	s, err := (Codex{}).Generate(context.Background(), Request{Sessions: oneSession})
	if err != nil {
		t.Fatalf("a Codex that predates the flag must still write the recap: %v", err)
	}
	if !strings.Contains(s.Content, "stub recap body") {
		t.Errorf("Content = %q", s.Content)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), "--ignore-user-config") || !strings.Contains(string(args), "--ephemeral\n") {
		t.Errorf("second attempt args = %q, want the same without the flag", args)
	}
	if calls, _ := os.ReadFile(argsFile + ".calls"); strings.Count(string(calls), "call") != 2 {
		t.Errorf("codex calls = %q, want two", calls)
	}
}

func TestCodexGenerate_KeepsTheErrorOutOfTheLog(t *testing.T) {
	stubCLI(t, "codex", `#!/bin/sh
/bin/cat >&2 <<'LOG'
OpenAI Codex v0.167.2
--------
model: gpt-6-luna
--------
2026-11-12T08:40:20.930327Z ERROR codex_models_manager::manager: failed to refresh available models: unexpected status 401 Unauthorized: Could not parse your authentication token.
ERROR: Reconnecting... 1/5
ERROR: Your access token could not be refreshed. Please log out and sign in again.
ERROR: Your access token could not be refreshed. Please log out and sign in again.
LOG
exit 1
`)
	_, err := (Codex{}).Generate(context.Background(), Request{Sessions: oneSession})
	want := "codex CLI failed (exit status 1): Your access token could not be refreshed. Please log out and sign in again."
	if err == nil || err.Error() != want {
		t.Errorf("err = %v\nwant %s", err, want)
	}
	if KindOf(err) != KindAuth {
		t.Errorf("KindOf = %v, want auth", KindOf(err))
	}
}

func TestCodexGenerate_ReadsTheKindFromCodexNotFromTheSessions(t *testing.T) {
	stubCLI(t, "codex", `#!/bin/sh
{
	echo "OpenAI Codex v0.169.0"
	echo "--------"
	echo "user"
	/bin/cat
	echo
	echo "ERROR: The 'gpt-4o' model is not supported when using Codex with a ChatGPT account."
} >&2
exit 1
`)
	sessions := []parser.Session{{
		ID: "s1", Tool: "codex", Project: "jogai",
		Messages: []parser.Message{
			{Role: "user", Content: "the login fails with\nERROR: authentication failed"},
			{Role: "assistant", Content: "I checked the usage policy page."},
		},
	}}
	_, err := (Codex{}).Generate(context.Background(), Request{Sessions: sessions})
	if KindOf(err) != KindConfig {
		t.Errorf("KindOf = %v, want config: %v", KindOf(err), err)
	}
}

func TestCodexFailure(t *testing.T) {
	tests := []struct{ name, log, want string }{
		{"last error line", "warning: x\nERROR: Reconnecting... 5/5\nERROR: model refused\n", "model refused"},
		{"no error line keeps the log", "boom\nmore", "boom\nmore"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		if got := codexFailure(tt.log); got != tt.want {
			t.Errorf("%s: codexFailure = %q, want %q", tt.name, got, tt.want)
		}
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
