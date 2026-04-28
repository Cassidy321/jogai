package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestRunWindow_DefaultsToPreviousDevDay(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	since, until, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 17, 5, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got since=%v until=%v", since, until)
	}
}

func TestRunWindow_ForSpecificDay(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-15"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	since, until, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 15, 5, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 16, 5, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got since=%v until=%v", since, until)
	}
}

func TestRunWindow_CalendarDayBoundary(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-15"}
	dayEnd := config.TimeOfDay{Hour: 0, Minute: 0}

	since, until, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got since=%v until=%v", since, until)
	}
}

func TestRunWindow_RejectsInvalidDateFormat(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "04/15/2026"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	_, _, err := cmd.window(now, dayEnd)
	if err == nil {
		t.Fatal("expected error for malformed date")
	}
	if !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("error should mention expected format, got: %v", err)
	}
}

func TestRunWindow_RejectsInProgressDay(t *testing.T) {
	// dev day 2026-04-18 runs from 04-18 05:00 to 04-19 05:00; at 14:00 it's still in progress.
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-18"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	_, _, err := cmd.window(now, dayEnd)
	if err == nil {
		t.Fatal("expected error for in-progress day")
	}
	if !strings.Contains(err.Error(), "not yet complete") {
		t.Errorf("error should mention incomplete dev day, got: %v", err)
	}
}

func TestRunWindow_RejectsFutureDay(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-20"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	_, _, err := cmd.window(now, dayEnd)
	if err == nil {
		t.Fatal("expected error for future day")
	}
}

func TestRunWindow_AcceptsLegacyFlagsButIgnoresThem(t *testing.T) {
	// Legacy --scheduled and --at flags from v0.4 plists must still parse but
	// should not affect the window (always computed from devday).
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Scheduled: true, At: "09:00"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	since, until, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 17, 5, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("legacy flags changed window behavior: since=%v until=%v", since, until)
	}
}

func TestBuildActiveParsers_DefaultsToClaude(t *testing.T) {
	cfg := &config.Config{}
	parsers, err := buildActiveParsers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsers) != 1 {
		t.Fatalf("got %d parsers, want 1", len(parsers))
	}
	if parsers[0].Name() != "claude-code" {
		t.Errorf("default should be claude-code, got %q", parsers[0].Name())
	}
}

func TestBuildActiveParsers_HonorsConfig(t *testing.T) {
	cfg := &config.Config{Sources: []string{"codex"}}
	parsers, err := buildActiveParsers(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsers) != 1 {
		t.Fatalf("got %d parsers, want 1", len(parsers))
	}
	if parsers[0].Name() != "codex" {
		t.Errorf("got %q, want codex", parsers[0].Name())
	}
}

func TestBuildActiveParsers_UnknownSource(t *testing.T) {
	cfg := &config.Config{Sources: []string{"cursor"}}
	_, err := buildActiveParsers(cfg)
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestSelectSummarizer_DefaultsToClaude(t *testing.T) {
	s := selectSummarizer("")
	if s.Name() != "claude" {
		t.Errorf("default summarizer = %q, want claude", s.Name())
	}
}

func TestSelectSummarizer_Codex(t *testing.T) {
	s := selectSummarizer("codex")
	if s.Name() != "codex" {
		t.Errorf("got %q, want codex", s.Name())
	}
}

func TestRun_MultiSourcePartialFailureWritesWarningHeader(t *testing.T) {
	// Stub Claude binary so CheckCLI passes and Generate returns a canned JSON response.
	bindir := t.TempDir()
	stub := `#!/bin/sh
/bin/cat <<'EOF'
{"result":"stub content","is_error":false,"total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}
EOF
`
	if err := os.WriteFile(filepath.Join(bindir, "claude"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bindir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Prepare HOME with jogai config, a claude session folder, and an unreadable codex day folder.
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Minimal Claude Code session.
	ccDir := filepath.Join(home, ".claude", "projects", "-tmp-test")
	if err := os.MkdirAll(ccDir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"user","sessionId":"s1","cwd":"/tmp/test","timestamp":"2026-04-21T10:00:00Z","message":{"role":"user","content":"hi"}}` + "\n" +
		`{"type":"assistant","sessionId":"s1","cwd":"/tmp/test","timestamp":"2026-04-21T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"hello"}]}}`
	if err := os.WriteFile(filepath.Join(ccDir, "s1.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	// Codex day dir: drop a stub file inside before chmod 0o000 so ReadDir surfaces a permission error.
	cxDay := filepath.Join(home, ".codex", "sessions", "2026", "04", "21")
	if err := os.MkdirAll(cxDay, 0o755); err != nil {
		t.Fatal(err)
	}
	stubFile := filepath.Join(cxDay, "rollout-stub.jsonl")
	if err := os.WriteFile(stubFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cxDay, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cxDay, 0o755) })

	outDir := filepath.Join(home, "recaps")
	cfgDir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{"output_dir":"` + outDir + `","day_end":"00:00","sources":["claude-code","codex"],"summarizer":"claude"}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &RunCmd{Day: "2026-04-21"}
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "2026-04-21.md"))
	if err != nil {
		t.Fatalf("expected recap file: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "> ⚠ codex:") {
		t.Errorf("expected codex warning blockquote in markdown:\n%s", body)
	}
	if !strings.Contains(body, "stub content") {
		t.Errorf("expected recap body from stub:\n%s", body)
	}
}
