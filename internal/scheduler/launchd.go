package scheduler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/summary"
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
		<string>{{.BinDir}}:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>
	</dict>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/bin/caffeinate</string>
		<string>-i</string>
		<string>-s</string>
		<string>{{.ExecPath}}</string>
		<string>run</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
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
	ExecPath string
	BinDir   string
	DayEnd   config.TimeOfDay
	LogDir   string
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

func generatePlist(dayEnd config.TimeOfDay, execPath, binDir, logDir string) ([]byte, error) {
	var buf bytes.Buffer
	if err := plistTmpl.Execute(&buf, plistData{
		ExecPath: execPath,
		BinDir:   binDir,
		DayEnd:   dayEnd,
		LogDir:   logDir,
	}); err != nil {
		return nil, fmt.Errorf("execute plist template: %w", err)
	}
	return buf.Bytes(), nil
}

func IsTempBinary(path string) bool {
	for _, marker := range []string{"/go-build", "/tmp/", "/var/folders/"} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

func (l *launchd) Install() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	if IsTempBinary(execPath) {
		return fmt.Errorf("cannot install schedule from a temporary binary (%s) — build and install jogai first", execPath)
	}
	plist, err := l.render(execPath)
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
	if err := l.reload(); err != nil {
		if hadOldPlist {
			_ = os.WriteFile(path, oldPlist, 0o644)
			_ = l.reload()
		} else {
			_ = os.Remove(path)
		}
		return err
	}
	return nil
}

// Repair never installs a schedule the user stopped, and keeps the binary
// already in the plist so a dev build run by hand cannot take over the
// schedule. Inside the job, reloading would kill the running recap: the new
// plist is picked up at the next login instead.
func (l *launchd) Repair() (bool, error) {
	current, err := os.ReadFile(l.plistPath())
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read plist: %w", err)
	}
	execPath := plistExecPath(current)
	if execPath == "" {
		return false, nil
	}
	want, err := l.render(execPath)
	if err != nil {
		return false, err
	}
	if bytes.Equal(current, want) {
		return false, nil
	}
	if err := os.WriteFile(l.plistPath(), want, 0o644); err != nil {
		return false, fmt.Errorf("write plist: %w", err)
	}
	if InJob() {
		return true, nil
	}
	return true, l.reload()
}

func (l *launchd) render(execPath string) ([]byte, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.DayEnd == nil {
		return nil, fmt.Errorf("dev day boundary not configured — run 'jogai init' to set it before scheduling")
	}
	binPath, err := summary.LookPath(summarizerBin(cfg))
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(l.configDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	return generatePlist(*cfg.DayEnd, execPath, filepath.Dir(binPath), logDir)
}

func (l *launchd) reload() error {
	_ = exec.Command("launchctl", "unload", l.plistPath()).Run()
	if err := exec.Command("launchctl", "load", l.plistPath()).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}
	return nil
}

func summarizerBin(cfg *config.Config) string {
	if cfg.Summarizer == summary.NameCodex {
		return summary.NameCodex
	}
	return summary.NameClaude
}

// Tied to plistTmpl: the jogai binary is the argument right after caffeinate's
// -s. If the template changes and this stops matching, Repair silently does nothing.
var execPathRe = regexp.MustCompile(`<string>-s</string>\s*<string>([^<]+)</string>`)

func plistExecPath(plist []byte) string {
	m := execPathRe.FindSubmatch(plist)
	if m == nil {
		return ""
	}
	return string(m[1])
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

func (l *launchd) Status() (Job, error) {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotConfigured) {
			return Job{}, nil
		}
		return Job{}, err
	}
	job := Job{At: cfg.DayEnd, Active: l.isLoaded()}
	if job.Active && cfg.DayEnd != nil {
		job.NextRun = nextRun(*cfg.DayEnd, time.Now())
	}
	return job, nil
}
