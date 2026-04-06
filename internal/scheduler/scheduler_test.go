package scheduler

import (
	"testing"
	"time"
)

func TestParseAtDaily(t *testing.T) {
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
			s, err := ParseAt("daily", tt.at)
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
			if s.Weekday != -1 {
				t.Errorf("weekday should be -1, got %d", s.Weekday)
			}
			if s.MonthDay != -1 {
				t.Errorf("monthday should be -1, got %d", s.MonthDay)
			}
		})
	}
}

func TestParseAtWeekly(t *testing.T) {
	tests := []struct {
		name    string
		at      string
		wantWD  int
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"monday", "mon:09:00", 1, 9, 0, false},
		{"friday evening", "fri:18:30", 5, 18, 30, false},
		{"sunday midnight", "sun:00:00", 0, 0, 0, false},
		{"saturday", "sat:12:00", 6, 12, 0, false},
		{"default", "", 1, 9, 0, false},
		{"wrong format", "09:00", 0, 0, 0, true},
		{"invalid day", "abc:09:00", 0, 0, 0, true},
		{"invalid hour", "mon:25:00", 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseAt("weekly", tt.at)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.Weekday != tt.wantWD || s.Hour != tt.wantH || s.Minute != tt.wantM {
				t.Errorf("got wd=%d %d:%02d, want wd=%d %d:%02d",
					s.Weekday, s.Hour, s.Minute, tt.wantWD, tt.wantH, tt.wantM)
			}
		})
	}
}

func TestParseAtMonthly(t *testing.T) {
	tests := []struct {
		name    string
		at      string
		wantDay int
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"first", "1st:09:00", 1, 9, 0, false},
		{"second", "2nd:18:30", 2, 18, 30, false},
		{"third", "3rd:12:00", 3, 12, 0, false},
		{"fifteenth", "15th:09:00", 15, 9, 0, false},
		{"default", "", 1, 9, 0, false},
		{"day zero", "0th:09:00", 0, 0, 0, true},
		{"day 32", "32nd:09:00", 0, 0, 0, true},
		{"wrong format", "09:00", 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseAt("monthly", tt.at)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.MonthDay != tt.wantDay || s.Hour != tt.wantH || s.Minute != tt.wantM {
				t.Errorf("got day=%d %d:%02d, want day=%d %d:%02d",
					s.MonthDay, s.Hour, s.Minute, tt.wantDay, tt.wantH, tt.wantM)
			}
		})
	}
}

func TestParseAtUnsupportedPeriod(t *testing.T) {
	_, err := ParseAt("session", "")
	if err == nil {
		t.Fatal("expected error for unsupported period")
	}
}

func TestNextRunDaily(t *testing.T) {
	// 2026-04-06 10:00 — already past 09:00 today
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0, Weekday: -1, MonthDay: -1}
	got := nextRun("daily", sched, now)
	want := time.Date(2026, 4, 7, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNextRunDailyNotYetPassed(t *testing.T) {
	// 2026-04-06 08:00 — not yet 09:00 today
	now := time.Date(2026, 4, 6, 8, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0, Weekday: -1, MonthDay: -1}
	got := nextRun("daily", sched, now)
	want := time.Date(2026, 4, 6, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNextRunWeekly(t *testing.T) {
	// 2026-04-06 is a Monday, 10:00 — past mon:09:00
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0, Weekday: 1, MonthDay: -1}
	got := nextRun("weekly", sched, now)
	want := time.Date(2026, 4, 13, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNextRunWeeklyDifferentDay(t *testing.T) {
	// 2026-04-06 is a Monday — Friday is in 4 days
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0, Weekday: 5, MonthDay: -1}
	got := nextRun("weekly", sched, now)
	want := time.Date(2026, 4, 10, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNextRunMonthly(t *testing.T) {
	// April 6 — 1st already passed
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0, Weekday: -1, MonthDay: 1}
	got := nextRun("monthly", sched, now)
	want := time.Date(2026, 5, 1, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNextRunMonthlyNotYetPassed(t *testing.T) {
	// April 6 — 15th is still ahead
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.Local)
	sched := Schedule{Hour: 9, Minute: 0, Weekday: -1, MonthDay: 15}
	got := nextRun("monthly", sched, now)
	want := time.Date(2026, 4, 15, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
