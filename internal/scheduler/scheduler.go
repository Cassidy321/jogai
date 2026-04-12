package scheduler

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

// Scheduler manages the OS-level daily scheduled job for jogai.
type Scheduler interface {
	Install(at string) error
	Uninstall() error
	Status() ([]Job, error)
}

// Job represents the scheduled daily recap.
type Job struct {
	At      string
	Active  bool
	NextRun time.Time
}

// Schedule holds the parsed components of an --at value.
type Schedule struct {
	Hour   int
	Minute int
}

var dailyRe = regexp.MustCompile(`^(\d{2}):(\d{2})$`)

// ResolveAt returns at with the default applied if empty.
func ResolveAt(at string) string {
	if at != "" {
		return at
	}
	return "09:00"
}

// ParseAt validates and parses an --at string.
func ParseAt(at string) (Schedule, error) {
	at = ResolveAt(at)

	m := dailyRe.FindStringSubmatch(at)
	if m == nil {
		return Schedule{}, fmt.Errorf("invalid schedule %q — expected HH:MM (e.g. 09:00)", at)
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	if h > 23 || min > 59 {
		return Schedule{}, fmt.Errorf("invalid time %q — hour 00-23, minute 00-59", at)
	}
	return Schedule{Hour: h, Minute: min}, nil
}

func nextRun(sched Schedule, now time.Time) time.Time {
	candidate := time.Date(now.Year(), now.Month(), now.Day(),
		sched.Hour, sched.Minute, 0, 0, now.Location())
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

// Window returns the fixed daily window anchored on the schedule time.
// The returned interval is [since, until).
func Window(sched Schedule, now time.Time) (since, until time.Time) {
	until = time.Date(now.Year(), now.Month(), now.Day(),
		sched.Hour, sched.Minute, 0, 0, now.Location())
	if until.After(now) {
		until = until.AddDate(0, 0, -1)
	}
	return until.AddDate(0, 0, -1), until
}

// New returns a Scheduler for the current OS.
func New() (Scheduler, error) {
	switch runtime.GOOS {
	case "darwin":
		return newLaunchd()
	default:
		return nil, fmt.Errorf("scheduling not supported on %s yet", runtime.GOOS)
	}
}
