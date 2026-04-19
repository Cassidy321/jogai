package scheduler

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.jogai.daily</string>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>{{.ClaudeDir}}:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>
	</dict>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/bin/caffeinate</string>
		<string>-i</string>
		<string>-s</string>
		<string>{{.ExecPath}}</string>
		<string>run</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>{{.DayEnd.Hour}}</integer>
		<key>Minute</key>
		<integer>{{.DayEnd.Minute}}</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>{{.LogDir}}/daily.out.log</string>
	<key>StandardErrorPath</key>
	<string>{{.LogDir}}/daily.err.log</string>
</dict>
</plist>
`))

const launchdLabel = "com.jogai.daily"

type plistData struct {
	ExecPath  string
	ClaudeDir string
	DayEnd    config.TimeOfDay
	LogDir    string
}

type launchd struct {
	agentsDir string
	configDir string
}

func newLaunchd() (*launchd, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	configDir, err := config.Dir()
	if err != nil {
		return nil, err
	}

	return &launchd{
		agentsDir: filepath.Join(home, "Library", "LaunchAgents"),
		configDir: configDir,
	}, nil
}

func (l *launchd) plistPath() string {
	return filepath.Join(l.agentsDir, launchdLabel+".plist")
}

func (l *launchd) isLoaded() bool {
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdLabel)
	cmd := exec.Command("launchctl", "print", target)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func generatePlist(dayEnd config.TimeOfDay, execPath, claudeDir, logDir string) ([]byte, error) {
	var buf bytes.Buffer
	if err := plistTmpl.Execute(&buf, plistData{
		ExecPath:  execPath,
		ClaudeDir: claudeDir,
		DayEnd:    dayEnd,
		LogDir:    logDir,
	}); err != nil {
		return nil, fmt.Errorf("execute plist template: %w", err)
	}
	return buf.Bytes(), nil
}

func isTempBinary(path string) bool {
	for _, marker := range []string{"/go-build", "/tmp/", "/var/folders/"} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

func (l *launchd) Install() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	if isTempBinary(execPath) {
		return fmt.Errorf("cannot install schedule from a temporary binary (%s) — build and install jogai first", execPath)
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found — install it from https://claude.com/product/claude-code")
	}

	logDir := filepath.Join(l.configDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	plist, err := generatePlist(cfg.DayEnd, execPath, filepath.Dir(claudePath), logDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(l.agentsDir, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	path := l.plistPath()
	oldPlist, hadOldPlist := backupFile(path)

	if err := os.WriteFile(path, plist, 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	_ = exec.Command("launchctl", "unload", path).Run()
	if err := exec.Command("launchctl", "load", path).Run(); err != nil {
		if hadOldPlist {
			_ = os.WriteFile(path, oldPlist, 0o644)
			_ = exec.Command("launchctl", "load", path).Run()
		} else {
			_ = os.Remove(path)
		}
		return fmt.Errorf("launchctl load: %w", err)
	}

	return nil
}

func backupFile(path string) ([]byte, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (l *launchd) Uninstall() error {
	path := l.plistPath()
	_ = exec.Command("launchctl", "unload", path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	// Remove legacy schedules.json from v0.4 if it exists.
	_ = os.Remove(filepath.Join(l.configDir, "schedules.json"))
	return nil
}

func (l *launchd) Status() ([]Job, error) {
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrNotConfigured {
			return []Job{{Active: false}}, nil
		}
		return nil, err
	}

	job := Job{
		At:     cfg.DayEnd.String(),
		Active: l.isLoaded(),
	}
	if job.Active {
		job.NextRun = nextRun(cfg.DayEnd, time.Now())
	}

	return []Job{job}, nil
}
