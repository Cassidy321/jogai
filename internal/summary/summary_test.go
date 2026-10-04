package summary

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/parser"
)

func TestBuildPrompt(t *testing.T) {
	day := time.Date(2026, 4, 5, 5, 0, 0, 0, time.UTC)
	sessions := []parser.Session{
		{
			ID:        "s1",
			Tool:      "claude-code",
			StartedAt: time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
			EndedAt:   time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
			Project:   "jogai",
			Messages: []parser.Message{
				{Role: "user", Content: "add a login page"},
				{Role: "assistant", Content: "I'll create a login page for you."},
			},
		},
	}

	prompt, err := buildPrompt(day, sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "1 AI coding session") {
		t.Error("prompt should mention session count")
	}
	if !strings.Contains(prompt, "daily recap") {
		t.Error("prompt should mention daily recap")
	}
	if !strings.Contains(prompt, "Do not include a document title/heading") {
		t.Error("prompt should forbid a generated title")
	}
	if !strings.Contains(prompt, day.Format("Monday 2 January 2006")) {
		t.Error("prompt should name the dev day")
	}
	if !strings.Contains(prompt, "jogai") {
		t.Error("prompt should include project name")
	}
	if !strings.Contains(prompt, "add a login page") {
		t.Error("prompt should include user message")
	}
	if !strings.Contains(prompt, "I'll create a login page") {
		t.Error("prompt should include assistant message")
	}
}

func TestBuildPromptMultipleSessions(t *testing.T) {
	day := time.Date(2026, 4, 5, 5, 0, 0, 0, time.UTC)
	sessions := []parser.Session{
		{
			ID:      "s1",
			Project: "jogai",
			Messages: []parser.Message{
				{Role: "user", Content: "first session"},
			},
		},
		{
			ID:      "s2",
			Project: "socadb",
			Messages: []parser.Message{
				{Role: "user", Content: "second session"},
			},
		},
	}

	prompt, err := buildPrompt(day, sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "2 AI coding session") {
		t.Error("prompt should mention 2 sessions")
	}
	if !strings.Contains(prompt, "jogai") {
		t.Error("prompt should contain first project name")
	}
	if !strings.Contains(prompt, "socadb") {
		t.Error("prompt should contain second project name")
	}
	if !strings.Contains(prompt, "<sessions>") {
		t.Error("prompt should wrap sessions in tags")
	}
	if !strings.Contains(prompt, "</sessions>") {
		t.Error("prompt should close sessions tag")
	}
}

func TestClaudeGenerateNoSessions(t *testing.T) {
	_, err := Claude{}.Generate(context.Background(), time.Time{}, nil)
	if err == nil {
		t.Error("expected error for empty sessions")
	}
}

func TestClaudeGenerate_MissingCLIReportsPath(t *testing.T) {
	noFallbackDirs(t)
	t.Setenv("PATH", "")
	sessions := []parser.Session{{
		ID: "s1", Tool: "claude-code", Project: "jogai",
		Messages: []parser.Message{{Role: "user", Content: "hi"}},
	}}
	_, err := Claude{}.Generate(context.Background(), time.Time{}, sessions)
	if err == nil {
		t.Fatal("expected error for missing claude CLI")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error should mention PATH, got: %v", err)
	}
}

func stubCLI(t *testing.T, bin, script string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, bin), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	argsFile = filepath.Join(dir, "args")
	t.Setenv("ARGS_FILE", argsFile)
	return argsFile
}

// Without it LookPath finds the real CLIs installed on the machine running the tests.
func noFallbackDirs(t *testing.T) {
	t.Helper()
	orig := fallbackDirs
	fallbackDirs = func() []string { return nil }
	t.Cleanup(func() { fallbackDirs = orig })
}

var oneSession = []parser.Session{{
	ID: "s1", Tool: "claude-code", Project: "jogai",
	Messages: []parser.Message{{Role: "user", Content: "hi"}},
}}

func TestClaudeGenerate_IsolatesTheSummarizer(t *testing.T) {
	argsFile := stubCLI(t, "claude", `#!/bin/sh
printf '%s\n' "$@" > "$ARGS_FILE"
/bin/cat > /dev/null
echo '{"result":"recap body","is_error":false}'
`)
	s, err := Claude{}.Generate(context.Background(), time.Now(), oneSession)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Content != "recap body" {
		t.Errorf("Content = %q", s.Content)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if !slices.Equal(got, claudeArgs) {
		t.Errorf("claude args = %q, want %q", got, claudeArgs)
	}
}

func TestClaudeGenerate_ClassifiesErrorResults(t *testing.T) {
	stubCLI(t, "claude", `#!/bin/sh
/bin/cat > /dev/null
echo '{"result":"API Error: Stream idle timeout - partial response received","is_error":true}'
exit 1
`)
	_, err := Claude{}.Generate(context.Background(), time.Now(), oneSession)
	if KindOf(err) != KindTransient {
		t.Errorf("KindOf(%v) = %v, want transient", err, KindOf(err))
	}
}

func TestClaudeGenerate_TimeoutIsTransient(t *testing.T) {
	stubCLI(t, "claude", "#!/bin/sh\nexec /bin/sleep 5\n")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := Claude{}.Generate(ctx, time.Now(), oneSession)
	if KindOf(err) != KindTransient {
		t.Errorf("KindOf(%v) = %v, want transient", err, KindOf(err))
	}
}

func TestLookPath_FallsBackToKnownDirs(t *testing.T) {
	t.Setenv("PATH", "")
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := fallbackDirs
	fallbackDirs = func() []string { return []string{dir} }
	t.Cleanup(func() { fallbackDirs = orig })

	got, err := LookPath("claude")
	if err != nil || got != bin {
		t.Fatalf("LookPath = (%q, %v), want %q", got, err, bin)
	}
}
