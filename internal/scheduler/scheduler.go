package scheduler

import (
	"fmt"
	"runtime"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

// Scheduler manages the OS-level daily scheduled job for jogai.
type Scheduler interface {
	Install() error
	Uninstall() error
	Status() ([]Job, error)
}

// Job represents the scheduled daily recap.
type Job struct {
	At      *config.TimeOfDay
	Active  bool
	NextRun time.Time
}

func nextRun(t config.TimeOfDay, now time.Time) time.Time {
	candidate := time.Date(now.Year(), now.Month(), now.Day(),
		t.Hour, t.Minute, 0, 0, now.Location())
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
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
