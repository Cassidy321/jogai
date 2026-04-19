// Package devday defines the dev-day window semantics used by jogai.
//
// A dev day is a 24h (clock-anchored) window in local time, bounded by the
// user-configured end-of-day hour. The window labeled "YYYY-MM-DD" runs from
// that date's boundary (inclusive) to the next day's boundary (exclusive).
//
// DST note: during spring-forward / fall-back days the window's real duration
// is 23h or 25h. Clock time is preserved; this is the intended behavior.
package devday

import (
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

const LabelFormat = "2006-01-02"

// Window returns the dev-day window that contains ref.
// The interval is [start, end); label is YYYY-MM-DD of start.
func Window(ref time.Time, dayEnd config.TimeOfDay) (start, end time.Time, label string) {
	y, m, d := ref.Date()
	todayBoundary := time.Date(y, m, d, dayEnd.Hour, dayEnd.Minute, 0, 0, ref.Location())
	if ref.Before(todayBoundary) {
		start = todayBoundary.AddDate(0, 0, -1)
	} else {
		start = todayBoundary
	}
	end = start.AddDate(0, 0, 1)
	label = start.Format(LabelFormat)
	return start, end, label
}

// Previous returns the dev-day window immediately before the one containing ref.
// This is what `jogai run` targets — the most recently completed dev day.
func Previous(ref time.Time, dayEnd config.TimeOfDay) (start, end time.Time, label string) {
	curStart, _, _ := Window(ref, dayEnd)
	end = curStart
	start = end.AddDate(0, 0, -1)
	label = start.Format(LabelFormat)
	return start, end, label
}

// FromDate returns the dev-day window labeled by date (only the Y/M/D components
// are used). Used for `jogai run --day YYYY-MM-DD`.
func FromDate(date time.Time, dayEnd config.TimeOfDay) (start, end time.Time, label string) {
	y, m, d := date.Date()
	start = time.Date(y, m, d, dayEnd.Hour, dayEnd.Minute, 0, 0, date.Location())
	end = start.AddDate(0, 0, 1)
	label = start.Format(LabelFormat)
	return start, end, label
}
