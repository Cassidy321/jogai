package devday

import (
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

func mustLoadParis(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatalf("load Europe/Paris: %v", err)
	}
	return loc
}

func TestWindow_CalendarDay(t *testing.T) {
	loc := time.UTC
	dayEnd := config.TimeOfDay{Hour: 0, Minute: 0}
	ref := time.Date(2026, 4, 17, 14, 0, 0, 0, loc)

	start, end, label := Window(ref, dayEnd)

	wantStart := time.Date(2026, 4, 17, 0, 0, 0, 0, loc)
	wantEnd := time.Date(2026, 4, 18, 0, 0, 0, 0, loc)
	if !start.Equal(wantStart) {
		t.Errorf("start = %v, want %v", start, wantStart)
	}
	if !end.Equal(wantEnd) {
		t.Errorf("end = %v, want %v", end, wantEnd)
	}
	if label != "2026-04-17" {
		t.Errorf("label = %q, want 2026-04-17", label)
	}
}

func TestWindow_AfterBoundary(t *testing.T) {
	loc := time.UTC
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}
	ref := time.Date(2026, 4, 17, 14, 0, 0, 0, loc)

	start, end, label := Window(ref, dayEnd)

	wantStart := time.Date(2026, 4, 17, 5, 0, 0, 0, loc)
	wantEnd := time.Date(2026, 4, 18, 5, 0, 0, 0, loc)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) || label != "2026-04-17" {
		t.Errorf("got (%v, %v, %q), want (%v, %v, 2026-04-17)", start, end, label, wantStart, wantEnd)
	}
}

func TestWindow_BeforeBoundary(t *testing.T) {
	loc := time.UTC
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}
	ref := time.Date(2026, 4, 18, 2, 0, 0, 0, loc)

	start, end, label := Window(ref, dayEnd)

	wantStart := time.Date(2026, 4, 17, 5, 0, 0, 0, loc)
	wantEnd := time.Date(2026, 4, 18, 5, 0, 0, 0, loc)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) || label != "2026-04-17" {
		t.Errorf("got (%v, %v, %q), want (%v, %v, 2026-04-17)", start, end, label, wantStart, wantEnd)
	}
}

func TestWindow_ExactlyAtBoundary(t *testing.T) {
	// ref exactly at the boundary belongs to the new dev day (half-open interval).
	loc := time.UTC
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}
	ref := time.Date(2026, 4, 18, 5, 0, 0, 0, loc)

	start, _, label := Window(ref, dayEnd)

	wantStart := time.Date(2026, 4, 18, 5, 0, 0, 0, loc)
	if !start.Equal(wantStart) || label != "2026-04-18" {
		t.Errorf("got (%v, %q), want (%v, 2026-04-18)", start, label, wantStart)
	}
}

func TestPrevious(t *testing.T) {
	loc := time.UTC
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	tests := []struct {
		name       string
		ref        time.Time
		wantStart  time.Time
		wantEnd    time.Time
		wantLabel  string
	}{
		{
			name:      "afternoon same day",
			ref:       time.Date(2026, 4, 18, 14, 0, 0, 0, loc),
			wantStart: time.Date(2026, 4, 17, 5, 0, 0, 0, loc),
			wantEnd:   time.Date(2026, 4, 18, 5, 0, 0, 0, loc),
			wantLabel: "2026-04-17",
		},
		{
			name:      "before boundary",
			ref:       time.Date(2026, 4, 18, 3, 0, 0, 0, loc),
			wantStart: time.Date(2026, 4, 16, 5, 0, 0, 0, loc),
			wantEnd:   time.Date(2026, 4, 17, 5, 0, 0, 0, loc),
			wantLabel: "2026-04-16",
		},
		{
			name:      "exactly at boundary",
			ref:       time.Date(2026, 4, 18, 5, 0, 0, 0, loc),
			wantStart: time.Date(2026, 4, 17, 5, 0, 0, 0, loc),
			wantEnd:   time.Date(2026, 4, 18, 5, 0, 0, 0, loc),
			wantLabel: "2026-04-17",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end, label := Previous(tc.ref, dayEnd)
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) || label != tc.wantLabel {
				t.Errorf("got (%v, %v, %q), want (%v, %v, %q)",
					start, end, label, tc.wantStart, tc.wantEnd, tc.wantLabel)
			}
		})
	}
}

func TestFromDate(t *testing.T) {
	loc := time.UTC
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	// Time-of-day of the input is ignored; only Y/M/D matters.
	date := time.Date(2026, 4, 15, 23, 59, 59, 0, loc)

	start, end, label := FromDate(date, dayEnd)

	wantStart := time.Date(2026, 4, 15, 5, 0, 0, 0, loc)
	wantEnd := time.Date(2026, 4, 16, 5, 0, 0, 0, loc)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) || label != "2026-04-15" {
		t.Errorf("got (%v, %v, %q), want (%v, %v, 2026-04-15)", start, end, label, wantStart, wantEnd)
	}
}

func TestWindow_DSTSpringForward(t *testing.T) {
	// Europe/Paris: 2026-03-29 02:00 → 03:00 (skipped hour). Dev day 2026-03-28
	// with dayEnd=00:00 runs from 2026-03-28 00:00 to 2026-03-29 00:00, all normal.
	// The one ending ACROSS the transition is 2026-03-29 00:00 → 2026-03-30 00:00 = 23h real.
	loc := mustLoadParis(t)
	dayEnd := config.TimeOfDay{Hour: 0, Minute: 0}
	ref := time.Date(2026, 3, 29, 12, 0, 0, 0, loc)

	start, end, label := Window(ref, dayEnd)

	if label != "2026-03-29" {
		t.Errorf("label = %q, want 2026-03-29", label)
	}
	duration := end.Sub(start)
	if duration != 23*time.Hour {
		t.Errorf("spring-forward window duration = %v, want 23h", duration)
	}
}

func TestWindow_DSTFallBack(t *testing.T) {
	// Europe/Paris: 2026-10-25 03:00 → 02:00 (repeated hour). Window 2026-10-25 00:00
	// → 2026-10-26 00:00 = 25h real.
	loc := mustLoadParis(t)
	dayEnd := config.TimeOfDay{Hour: 0, Minute: 0}
	ref := time.Date(2026, 10, 25, 12, 0, 0, 0, loc)

	start, end, label := Window(ref, dayEnd)

	if label != "2026-10-25" {
		t.Errorf("label = %q, want 2026-10-25", label)
	}
	duration := end.Sub(start)
	if duration != 25*time.Hour {
		t.Errorf("fall-back window duration = %v, want 25h", duration)
	}
}

func TestPrevious_ThenWindow_ConsistentWithFromDate(t *testing.T) {
	// Previous(ref) should produce the same window as FromDate(<label of Previous>).
	loc := time.UTC
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}
	ref := time.Date(2026, 4, 18, 14, 0, 0, 0, loc)

	pStart, pEnd, pLabel := Previous(ref, dayEnd)

	labelAsDate, err := time.ParseInLocation(LabelFormat, pLabel, loc)
	if err != nil {
		t.Fatalf("parse label: %v", err)
	}
	fStart, fEnd, fLabel := FromDate(labelAsDate, dayEnd)

	if !pStart.Equal(fStart) || !pEnd.Equal(fEnd) || pLabel != fLabel {
		t.Errorf("Previous and FromDate disagree: Previous=(%v,%v,%q), FromDate=(%v,%v,%q)",
			pStart, pEnd, pLabel, fStart, fEnd, fLabel)
	}
}
