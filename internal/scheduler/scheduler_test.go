package scheduler

import (
	"testing"
	"time"
)

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %s: %v", name, err)
	}
	return loc
}

func TestParseAt(t *testing.T) {
	tests := []struct {
		name    string
		at      string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"explicit", "09:00", 9, 0, false},
		{"evening", "18:30", 18, 30, false},
		{"midnight", "00:00", 0, 0, false},
		{"end of day", "23:59", 23, 59, false},
		{"default", "", 9, 0, false},
		{"invalid hour", "25:00", 0, 0, true},
		{"invalid minute", "12:60", 0, 0, true},
		{"single digit", "9:00", 0, 0, true},
		{"wrong format", "mon:09:00", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseAt(tt.at)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.Hour != tt.wantH || s.Minute != tt.wantM {
				t.Errorf("got %d:%02d, want %d:%02d", s.Hour, s.Minute, tt.wantH, tt.wantM)
			}
		})
	}
}

func TestNextRunAlreadyPassed(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0}
	got := nextRun(sched, now)
	want := time.Date(2026, 4, 7, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNextRunNotYetPassed(t *testing.T) {
	now := time.Date(2026, 4, 6, 8, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0}
	got := nextRun(sched, now)
	want := time.Date(2026, 4, 6, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWindowAfterAnchor(t *testing.T) {
	now := time.Date(2026, 4, 11, 5, 7, 0, 0, time.Local)
	sched := Schedule{Hour: 5, Minute: 0}
	since, until := Window(sched, now)

	wantSince := time.Date(2026, 4, 10, 5, 0, 0, 0, time.Local)
	wantUntil := time.Date(2026, 4, 11, 5, 0, 0, 0, time.Local)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got [%v, %v), want [%v, %v)", since, until, wantSince, wantUntil)
	}
}

func TestWindowBeforeAnchor(t *testing.T) {
	now := time.Date(2026, 4, 11, 4, 59, 0, 0, time.Local)
	sched := Schedule{Hour: 5, Minute: 0}
	since, until := Window(sched, now)

	wantSince := time.Date(2026, 4, 9, 5, 0, 0, 0, time.Local)
	wantUntil := time.Date(2026, 4, 10, 5, 0, 0, 0, time.Local)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got [%v, %v), want [%v, %v)", since, until, wantSince, wantUntil)
	}
}

func TestWindowPreservesAnchorAcrossSpringDST(t *testing.T) {
	loc := mustLocation(t, "Europe/Paris")
	now := time.Date(2026, 3, 29, 9, 30, 0, 0, loc)
	sched := Schedule{Hour: 9, Minute: 0}
	since, until := Window(sched, now)

	wantSince := time.Date(2026, 3, 28, 9, 0, 0, 0, loc)
	wantUntil := time.Date(2026, 3, 29, 9, 0, 0, 0, loc)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got [%v, %v), want [%v, %v)", since, until, wantSince, wantUntil)
	}
}

func TestWindowPreservesAnchorAcrossFallDST(t *testing.T) {
	loc := mustLocation(t, "Europe/Paris")
	now := time.Date(2026, 10, 25, 9, 30, 0, 0, loc)
	sched := Schedule{Hour: 9, Minute: 0}
	since, until := Window(sched, now)

	wantSince := time.Date(2026, 10, 24, 9, 0, 0, 0, loc)
	wantUntil := time.Date(2026, 10, 25, 9, 0, 0, 0, loc)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got [%v, %v), want [%v, %v)", since, until, wantSince, wantUntil)
	}
}
