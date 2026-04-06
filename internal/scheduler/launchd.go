package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	<key>ProgramArguments</key>
	<array>
		<string>{{.ExecPath}}</string>
		<string>run</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>{{.Schedule.Hour}}</integer>
		<key>Minute</key>
		<integer>{{.Schedule.Minute}}</integer>
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
	Schedule Schedule
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
	err := exec.Command("launchctl", "list", launchdLabel).Run()
	return err == nil
}

func generatePlist(sched Schedule, execPath, logDir string) ([]byte, error) {
	var buf bytes.Buffer
	if err := plistTmpl.Execute(&buf, plistData{
		ExecPath: execPath,
		Schedule: sched,
		LogDir:   logDir,
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

func (l *launchd) schedulesPath() string {
	return filepath.Join(l.configDir, "schedules.json")
}

func (l *launchd) loadAt() (string, bool) {
	data, err := os.ReadFile(l.schedulesPath())
	if err != nil {
		return "", false
	}
	var info map[string]string
	if err := json.Unmarshal(data, &info); err != nil {
		return "", false
	}
	at, ok := info["at"]
	return at, ok
}

func (l *launchd) saveAt(at string) error {
	if err := os.MkdirAll(l.configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(map[string]string{"at": at}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.schedulesPath(), data, 0o644)
}

func (l *launchd) removeAt() {
	os.Remove(l.schedulesPath())
}

func (l *launchd) Install(at string) error {
	sched, err := ParseAt(at)
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

	logDir := filepath.Join(l.configDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	plist, err := generatePlist(sched, execPath, logDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(l.agentsDir, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	resolvedAt := ResolveAt(at)
	oldAt, hadOldMeta := l.loadAt()

	if err := l.saveAt(resolvedAt); err != nil {
		return fmt.Errorf("save schedule metadata: %w", err)
	}

	path := l.plistPath()
	oldPlist, hadOldPlist := backupFile(path)

	if err := os.WriteFile(path, plist, 0o644); err != nil {
		l.rollbackMeta(oldAt, hadOldMeta)
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
		l.rollbackMeta(oldAt, hadOldMeta)
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

func (l *launchd) rollbackMeta(oldAt string, hadOld bool) {
	if hadOld {
		_ = l.saveAt(oldAt)
	} else {
		l.removeAt()
	}
}

func (l *launchd) Uninstall() error {
	path := l.plistPath()
	_ = exec.Command("launchctl", "unload", path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	l.removeAt()
	return nil
}

func (l *launchd) Status() ([]Job, error) {
	at, hasInfo := l.loadAt()
	loaded := l.isLoaded()

	job := Job{Active: loaded}
	if hasInfo {
		job.At = at
		if sched, err := ParseAt(at); err == nil {
			job.NextRun = nextRun(sched, time.Now())
		}
	}

	return []Job{job}, nil
}
