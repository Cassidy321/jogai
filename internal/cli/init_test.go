package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestDefaultOutputDir(t *testing.T) {
	tests := []struct {
		name     string
		existing *config.Config
		fallback string
		want     string
	}{
		{"no existing", nil, "/home/jogai-recaps", "/home/jogai-recaps"},
		{"empty existing", &config.Config{}, "/home/jogai-recaps", "/home/jogai-recaps"},
		{"existing value", &config.Config{OutputDir: "/obsidian/CC"}, "/home/jogai-recaps", "/obsidian/CC"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultOutputDir(tc.existing, tc.fallback); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultDayEnd(t *testing.T) {
	tod5 := config.TimeOfDay{Hour: 5, Minute: 0}
	tod1913 := config.TimeOfDay{Hour: 19, Minute: 13}
	tests := []struct {
		name     string
		existing *config.Config
		want     string
	}{
		{"no existing", nil, "00:00"},
		{"unconfigured day_end", &config.Config{}, "00:00"},
		{"existing 05:00", &config.Config{DayEnd: &tod5}, "05:00"},
		{"existing 19:13", &config.Config{DayEnd: &tod1913}, "19:13"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultDayEnd(tc.existing); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home := "/Users/cassidy"
	tests := []struct {
		in   string
		want string
	}{
		{"~/notes", "/Users/cassidy/notes"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~notes", "~notes"},
		{"~/", "/Users/cassidy"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := expandHome(tc.in, home); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProbeWriteAccess_Success(t *testing.T) {
	dir := t.TempDir()
	if err := probeWriteAccess(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == ".jogai-write-test" {
			t.Errorf("sentinel file not cleaned up")
		}
	}
}

func TestProbeWriteAccess_MissingDirectory(t *testing.T) {
	if err := probeWriteAccess(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestUpdatedConfig_KeepsWhatTheWizardDoesNotAsk(t *testing.T) {
	off := false
	existing := &config.Config{
		OutputDir:  "/old",
		DayEnd:     &config.TimeOfDay{Hour: 0},
		Sources:    []string{"claude-code"},
		Summarizer: "claude",
		AutoUpdate: &off,
	}
	got := updatedConfig(existing, "/new", config.TimeOfDay{Hour: 5}, []string{"claude-code", "codex"}, "codex")
	if got.AutoUpdate == nil || *got.AutoUpdate {
		t.Errorf("auto_update = %v, want it kept off", got.AutoUpdate)
	}
	if got.OutputDir != "/new" || got.DayEnd.Hour != 5 || len(got.Sources) != 2 || got.Summarizer != "codex" {
		t.Errorf("config = %+v, want the new answers", got)
	}
	if existing.OutputDir != "/old" || existing.DayEnd.Hour != 0 {
		t.Errorf("the saved config must not be modified in place: %+v", existing)
	}
	t.Setenv("HOME", t.TempDir())
	if err := config.Save(got); err != nil {
		t.Fatal(err)
	}
	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.AutoUpdateEnabled() {
		t.Error("auto_update: false must still be in the saved file")
	}
}

func TestUpdatedConfig_FirstSetup(t *testing.T) {
	got := updatedConfig(nil, "/recaps", config.TimeOfDay{Hour: 5}, []string{"claude-code"}, "claude")
	if got.OutputDir != "/recaps" || got.DayEnd.Hour != 5 || got.Summarizer != "claude" || got.AutoUpdate != nil {
		t.Errorf("config = %+v", got)
	}
	if !got.AutoUpdateEnabled() {
		t.Error("updates stay on by default")
	}
}

func TestNeedsClaudeToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	if !needsClaudeToken("claude", true, now) {
		t.Error("claude without a token must be offered one")
	}
	if needsClaudeToken("codex", true, now) {
		t.Error("codex has no token to offer")
	}
	if needsClaudeToken("claude", false, now) {
		t.Error("no offer where the token cannot be kept")
	}
	dir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "claude_token.json"), []byte(`{"created_at":"2026-11-12T10:00:00Z"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if needsClaudeToken("claude", true, now) {
		t.Error("a fresh token must not be offered again")
	}
	if !needsClaudeToken("claude", true, now.AddDate(0, 11, 15)) {
		t.Error("a token about to expire must be offered again")
	}
}

func TestDetectedSources_OnlyClaude(t *testing.T) {
	det := detectedSources{claudeCode: true, codex: false}
	got := det.names()
	if len(got) != 1 || got[0] != "claude-code" {
		t.Errorf("got %v, want [claude-code]", got)
	}
}

func TestDetectedSources_Both(t *testing.T) {
	det := detectedSources{claudeCode: true, codex: true}
	got := det.names()
	if len(got) != 2 {
		t.Fatalf("got %v, want 2", got)
	}
}

func TestDetectedSummarizers_PickOnlyOne(t *testing.T) {
	got := resolveSummarizer(detectedSummarizers{claude: true, codex: false}, "")
	if got != "claude" {
		t.Errorf("got %q, want claude", got)
	}
	got = resolveSummarizer(detectedSummarizers{claude: false, codex: true}, "")
	if got != "codex" {
		t.Errorf("got %q, want codex", got)
	}
}

func TestDetectedSummarizers_KeepsExistingWhenValid(t *testing.T) {
	got := resolveSummarizer(detectedSummarizers{claude: true, codex: true}, "codex")
	if got != "codex" {
		t.Errorf("got %q, want codex", got)
	}
}
