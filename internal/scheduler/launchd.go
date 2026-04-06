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
	<string>com.jogai.{{.Period}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.ExecPath}}</string>
		<string>run</string>
		<string>--period</string>
		<string>{{.Period}}</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>{{.Schedule.Hour}}</integer>
		<key>Minute</key>
		<integer>{{.Schedule.Minute}}</integer>
{{- if ge .Schedule.Weekday 0}}
		<key>Weekday</key>
		<integer>{{.Schedule.Weekday}}</integer>
{{- end}}
{{- if ge .Schedule.MonthDay 1}}
		<key>Day</key>
		<integer>{{.Schedule.MonthDay}}</integer>
{{- end}}
	</dict>
	<key>StandardOutPath</key>
	<string>{{.LogDir}}/{{.Period}}.out.log</string>
	<key>StandardErrorPath</key>
	<string>{{.LogDir}}/{{.Period}}.err.log</string>
</dict>
</plist>
`))

type plistData struct {
	Period   string
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

func (l *launchd) label(period string) string {
	return fmt.Sprintf("com.jogai.%s", period)
}

func (l *launchd) plistPath(period string) string {
	return filepath.Join(l.agentsDir, l.label(period)+".plist")
}

// isLoaded checks if a launchd job is actually loaded by querying launchctl.
func (l *launchd) isLoaded(period string) bool {
	err := exec.Command("launchctl", "list", l.label(period)).Run()
	return err == nil
}

func generatePlist(period string, sched Schedule, execPath, logDir string) ([]byte, error) {
	var buf bytes.Buffer
	if err := plistTmpl.Execute(&buf, plistData{
		Period:   period,
		ExecPath: execPath,
		Schedule: sched,
		LogDir:   logDir,
	}); err != nil {
		return nil, fmt.Errorf("execute plist template: %w", err)
	}
	return buf.Bytes(), nil
}

// isTempBinary returns true if the executable path looks like a go run temp binary.
func isTempBinary(path string) bool {
	for _, marker := range []string{"/go-build", "/tmp/", "/var/folders/"} {
		if strings.Contains(path, marker) {
			return true
		}
	}
	return false
}

// scheduleInfo maps period to at string, stored in schedules.json.
type scheduleInfo map[string]string

func (l *launchd) schedulesPath() string {
	return filepath.Join(l.configDir, "schedules.json")
}

func (l *launchd) loadScheduleInfo() (scheduleInfo, error) {
	data, err := os.ReadFile(l.schedulesPath())
	if err != nil {
		if os.IsNotExist(err) {
			return scheduleInfo{}, nil
		}
		return nil, err
	}
	var info scheduleInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return info, nil
}

func (l *launchd) saveScheduleInfo(period, at string) error {
	info, err := l.loadScheduleInfo()
	if err != nil {
		return fmt.Errorf("load existing schedules: %w", err)
	}
	info[period] = at

	if err := os.MkdirAll(l.configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.schedulesPath(), data, 0o644)
}

func (l *launchd) removeScheduleInfo(period string) error {
	info, err := l.loadScheduleInfo()
	if err != nil {
		return nil
	}
	delete(info, period)
	if len(info) == 0 {
		os.Remove(l.schedulesPath())
		return nil
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(l.schedulesPath(), data, 0o644)
}

func (l *launchd) removeAllScheduleInfo() error {
	os.Remove(l.schedulesPath())
	return nil
}

func (l *launchd) Install(period, at string) error {
	sched, err := ParseAt(period, at)
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

	plist, err := generatePlist(period, sched, execPath, logDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(l.agentsDir, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	// Capture old metadata for rollback.
	resolvedAt := ResolveAt(period, at)
	oldAt, hadOldMeta := l.getScheduleAt(period)

	// Save metadata first — if this fails, we haven't touched launchd.
	if err := l.saveScheduleInfo(period, resolvedAt); err != nil {
		return fmt.Errorf("save schedule metadata: %w", err)
	}

	path := l.plistPath(period)

	// Backup existing plist for rollback on failure.
	oldPlist, hadOldPlist := l.backupPlist(path)

	if err := os.WriteFile(path, plist, 0o644); err != nil {
		l.rollbackMeta(period, oldAt, hadOldMeta)
		return fmt.Errorf("write plist: %w", err)
	}

	// Unload old job then load new one.
	_ = exec.Command("launchctl", "unload", path).Run()
	if err := exec.Command("launchctl", "load", path).Run(); err != nil {
		// Rollback: restore old plist and reload, or clean up.
		if hadOldPlist {
			_ = os.WriteFile(path, oldPlist, 0o644)
			_ = exec.Command("launchctl", "load", path).Run()
		} else {
			_ = os.Remove(path)
		}
		l.rollbackMeta(period, oldAt, hadOldMeta)
		return fmt.Errorf("launchctl load: %w", err)
	}

	return nil
}

func (l *launchd) backupPlist(path string) ([]byte, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (l *launchd) getScheduleAt(period string) (string, bool) {
	info, err := l.loadScheduleInfo()
	if err != nil {
		return "", false
	}
	at, ok := info[period]
	return at, ok
}

// rollbackMeta restores the previous metadata value for a period.
func (l *launchd) rollbackMeta(period, oldAt string, hadOld bool) {
	if hadOld {
		_ = l.saveScheduleInfo(period, oldAt)
	} else {
		_ = l.removeScheduleInfo(period)
	}
}

func (l *launchd) Uninstall(period string) error {
	if period == "" {
		for _, p := range AllPeriods {
			if err := l.uninstallOne(p); err != nil {
				return err
			}
		}
		return l.removeAllScheduleInfo()
	}
	if err := l.uninstallOne(period); err != nil {
		return err
	}
	return l.removeScheduleInfo(period)
}

func (l *launchd) uninstallOne(period string) error {
	path := l.plistPath(period)
	_ = exec.Command("launchctl", "unload", path).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist %s: %w", period, err)
	}
	return nil
}

func (l *launchd) Status() ([]Job, error) {
	info, err := l.loadScheduleInfo()
	if err != nil {
		return nil, fmt.Errorf("load schedule info: %w", err)
	}

	var jobs []Job
	for _, period := range AllPeriods {
		job := Job{Period: period, Active: l.isLoaded(period)}
		if at, ok := info[period]; ok {
			job.At = at
			if sched, parseErr := ParseAt(period, at); parseErr == nil {
				job.NextRun = nextRun(period, sched, time.Now())
			}
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
