package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestRunWindow_DefaultsToPreviousDevDay(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	since, until, recapDate, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 17, 5, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) || !recapDate.Equal(wantSince) {
		t.Fatalf("got since=%v until=%v recapDate=%v", since, until, recapDate)
	}
}

func TestRunWindow_ForSpecificDay(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-15"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	since, until, recapDate, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 15, 5, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 16, 5, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) || !recapDate.Equal(wantSince) {
		t.Fatalf("got since=%v until=%v recapDate=%v", since, until, recapDate)
	}
}

func TestRunWindow_CalendarDayBoundary(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-15"}
	dayEnd := config.TimeOfDay{Hour: 0, Minute: 0}

	since, until, _, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("got since=%v until=%v", since, until)
	}
}

func TestRunWindow_RejectsInvalidDateFormat(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "04/15/2026"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	_, _, _, err := cmd.window(now, dayEnd)
	if err == nil {
		t.Fatal("expected error for malformed date")
	}
	if !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("error should mention expected format, got: %v", err)
	}
}

func TestRunWindow_RejectsInProgressDay(t *testing.T) {
	// dev day 2026-04-18 runs from 04-18 05:00 to 04-19 05:00; at 14:00 it's still in progress.
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-18"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	_, _, _, err := cmd.window(now, dayEnd)
	if err == nil {
		t.Fatal("expected error for in-progress day")
	}
	if !strings.Contains(err.Error(), "not yet complete") {
		t.Errorf("error should mention incomplete dev day, got: %v", err)
	}
}

func TestRunWindow_RejectsFutureDay(t *testing.T) {
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-20"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	_, _, _, err := cmd.window(now, dayEnd)
	if err == nil {
		t.Fatal("expected error for future day")
	}
}

func TestRunWindow_AcceptsLegacyFlagsButIgnoresThem(t *testing.T) {
	// Legacy --scheduled and --at flags from v0.4 plists must still parse but
	// should not affect the window (always computed from devday).
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	cmd := RunCmd{Scheduled: true, At: "09:00"}
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}

	since, until, _, err := cmd.window(now, dayEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 17, 5, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 18, 5, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) {
		t.Fatalf("legacy flags changed window behavior: since=%v until=%v", since, until)
	}
}
