package scheduler

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestIsTempBinary(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/bin/jogai", false},
		{"/usr/local/bin/jogai", false},
		{"/Users/cassidy/jogai/bin/jogai", false},
		{"/var/folders/xx/yy/T/go-build123/jogai", true},
		{"/tmp/go-build456/exe/jogai", true},
		{"/private/var/folders/zz/T/go-build789/jogai", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsTempBinary(tt.path); got != tt.want {
				t.Errorf("IsTempBinary(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestGeneratePlist(t *testing.T) {
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}
	plist, err := generatePlist(dayEnd, "/usr/local/bin/jogai", "/Users/test/.local/bin", "/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	s := string(plist)

	mustContain := []string{
		"<string>com.jogai.daily</string>",
		"<string>/usr/bin/caffeinate</string>",
		"<string>/usr/local/bin/jogai</string>",
		"<string>/Users/test/.local/bin:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>",
		"<string>run</string>",
		"<key>Hour</key>",
		"<integer>5</integer>",
		"<key>Minute</key>",
		"<integer>0</integer>",
		"<string>/tmp/logs/daily.out.log</string>",
		"<string>/tmp/logs/daily.err.log</string>",
		"<key>RunAtLoad</key>",
	}
	for _, want := range mustContain {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q", want)
		}
	}

	mustNotContain := []string{"--scheduled", "--at", "--period"}
	for _, bad := range mustNotContain {
		if strings.Contains(s, bad) {
			t.Errorf("plist should not contain %q", bad)
		}
	}
}

func TestPlistExecPath(t *testing.T) {
	plist, err := generatePlist(config.TimeOfDay{Hour: 5}, "/opt/homebrew/bin/jogai", "/bin", "/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	if got := plistExecPath(plist); got != "/opt/homebrew/bin/jogai" {
		t.Errorf("plistExecPath = %q", got)
	}
	if got := plistExecPath([]byte("<plist/>")); got != "" {
		t.Errorf("plistExecPath(no program) = %q, want empty", got)
	}
}

// XPC_SERVICE_NAME makes Repair behave as inside the launchd job, so it never calls launchctl.
func setupRepair(t *testing.T) (l *launchd, binDir string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XPC_SERVICE_NAME", launchdLabel)
	binDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	cfgDir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"output_dir":"/tmp/out","day_end":"05:00"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	l = &launchd{agentsDir: filepath.Join(home, "LaunchAgents"), configDir: cfgDir}
	if err := os.MkdirAll(l.agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return l, binDir
}

func TestRepair_RewritesADriftedPlistKeepingItsBinary(t *testing.T) {
	l, binDir := setupRepair(t)
	logDir := filepath.Join(l.configDir, "logs")
	stale, err := generatePlist(config.TimeOfDay{Hour: 6}, "/opt/homebrew/bin/jogai", binDir, logDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(l.plistPath(), stale, 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := l.Repair()
	if err != nil || !changed {
		t.Fatalf("Repair = (%v, %v), want (true, nil)", changed, err)
	}
	got, err := os.ReadFile(l.plistPath())
	if err != nil {
		t.Fatal(err)
	}
	want, err := generatePlist(config.TimeOfDay{Hour: 5}, "/opt/homebrew/bin/jogai", binDir, logDir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("repaired plist:\n%s\nwant:\n%s", got, want)
	}

	if changed, err := l.Repair(); err != nil || changed {
		t.Errorf("second Repair = (%v, %v), want (false, nil)", changed, err)
	}
}

func TestRepair_LeavesAStoppedScheduleAlone(t *testing.T) {
	l, _ := setupRepair(t)
	if changed, err := l.Repair(); err != nil || changed {
		t.Errorf("Repair without a plist = (%v, %v), want (false, nil)", changed, err)
	}
}
