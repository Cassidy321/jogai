package scheduler

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	PeriodDaily   = "daily"
	PeriodWeekly  = "weekly"
	PeriodMonthly = "monthly"
)

var AllPeriods = []string{PeriodDaily, PeriodWeekly, PeriodMonthly}

// Scheduler manages OS-level scheduled jobs for jogai.
type Scheduler interface {
	Install(period string, at string) error
	Uninstall(period string) error // period="" removes all
	Status() ([]Job, error)
}

// Job represents a single scheduled recap.
type Job struct {
	Period  string
	At      string
	Active  bool
	NextRun time.Time
}

// Schedule holds the parsed components of an --at value.
type Schedule struct {
	Hour     int
	Minute   int
	Weekday  int // 0=Sunday..6=Saturday, -1 if not applicable
	MonthDay int // 1-31, -1 if not applicable
}

var (
	dailyRe   = regexp.MustCompile(`^(\d{2}):(\d{2})$`)
	weeklyRe  = regexp.MustCompile(`^(mon|tue|wed|thu|fri|sat|sun):(\d{2}):(\d{2})$`)
	monthlyRe = regexp.MustCompile(`^(\d{1,2})(st|nd|rd|th):(\d{2}):(\d{2})$`)

	weekdays = map[string]int{
		"sun": 0, "mon": 1, "tue": 2, "wed": 3,
		"thu": 4, "fri": 5, "sat": 6,
	}
)

// ResolveAt returns at with defaults applied for the given period.
func ResolveAt(period, at string) string {
	if at != "" {
		return at
	}
	switch period {
	case PeriodDaily:
		return "09:00"
	case PeriodWeekly:
		return "mon:09:00"
	case PeriodMonthly:
		return "1st:09:00"
	}
	return at
}

// ParseAt validates and parses an --at string for the given period.
func ParseAt(period, at string) (Schedule, error) {
	at = ResolveAt(period, at)

	switch period {
	case PeriodDaily:
		return parseDaily(at)
	case PeriodWeekly:
		return parseWeekly(at)
	case PeriodMonthly:
		return parseMonthly(at)
	default:
		return Schedule{}, fmt.Errorf("unsupported period for scheduling: %s", period)
	}
}

func parseHHMM(hStr, mStr, context string) (int, int, error) {
	h, _ := strconv.Atoi(hStr)
	m, _ := strconv.Atoi(mStr)
	if h > 23 || m > 59 {
		return 0, 0, fmt.Errorf("invalid time in %q — hour 00-23, minute 00-59", context)
	}
	return h, m, nil
}

func parseDaily(at string) (Schedule, error) {
	m := dailyRe.FindStringSubmatch(at)
	if m == nil {
		return Schedule{}, fmt.Errorf("invalid daily schedule %q — expected HH:MM (e.g. 09:00)", at)
	}
	h, min, err := parseHHMM(m[1], m[2], at)
	if err != nil {
		return Schedule{}, err
	}
	return Schedule{Hour: h, Minute: min, Weekday: -1, MonthDay: -1}, nil
}

func parseWeekly(at string) (Schedule, error) {
	m := weeklyRe.FindStringSubmatch(strings.ToLower(at))
	if m == nil {
		return Schedule{}, fmt.Errorf("invalid weekly schedule %q — expected day:HH:MM (e.g. mon:09:00)", at)
	}
	wd, ok := weekdays[m[1]]
	if !ok {
		return Schedule{}, fmt.Errorf("invalid weekday %q", m[1])
	}
	h, min, err := parseHHMM(m[2], m[3], at)
	if err != nil {
		return Schedule{}, err
	}
	return Schedule{Hour: h, Minute: min, Weekday: wd, MonthDay: -1}, nil
}

func parseMonthly(at string) (Schedule, error) {
	m := monthlyRe.FindStringSubmatch(strings.ToLower(at))
	if m == nil {
		return Schedule{}, fmt.Errorf("invalid monthly schedule %q — expected Nth:HH:MM (e.g. 1st:09:00)", at)
	}
	day, _ := strconv.Atoi(m[1])
	if day < 1 || day > 31 {
		return Schedule{}, fmt.Errorf("invalid day %d — must be 1-31", day)
	}
	h, min, err := parseHHMM(m[3], m[4], at)
	if err != nil {
		return Schedule{}, err
	}
	return Schedule{Hour: h, Minute: min, Weekday: -1, MonthDay: day}, nil
}

// nextRun computes the next occurrence of the given schedule after now.
func nextRun(period string, sched Schedule, now time.Time) time.Time {
	candidate := time.Date(now.Year(), now.Month(), now.Day(),
		sched.Hour, sched.Minute, 0, 0, now.Location())

	switch period {
	case PeriodDaily:
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 0, 1)
		}
	case PeriodWeekly:
		daysUntil := (sched.Weekday - int(candidate.Weekday()) + 7) % 7
		candidate = candidate.AddDate(0, 0, daysUntil)
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 0, 7)
		}
	case PeriodMonthly:
		candidate = time.Date(now.Year(), now.Month(), sched.MonthDay,
			sched.Hour, sched.Minute, 0, 0, now.Location())
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 1, 0)
		}
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
