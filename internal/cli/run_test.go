package cli

import (
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/summary"
)

func TestRunTimeWindowDefaultsToLast24Hours(t *testing.T) {
	now := time.Date(2026, 4, 11, 15, 31, 0, 0, time.UTC)
	cmd := RunCmd{}

	since, until, recapDate, kind, err := cmd.timeWindow(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 10, 15, 31, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(now) || !recapDate.Equal(now) {
		t.Fatalf("got since=%v until=%v recapDate=%v", since, until, recapDate)
	}
	if kind != summary.KindLast24h {
		t.Fatalf("got kind=%q", kind)
	}
}

func TestRunTimeWindowForSpecificDay(t *testing.T) {
	now := time.Date(2026, 4, 11, 15, 31, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-10"}

	since, until, recapDate, kind, err := cmd.timeWindow(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) || !recapDate.Equal(wantSince) {
		t.Fatalf("got since=%v until=%v recapDate=%v", since, until, recapDate)
	}
	if kind != summary.KindDay {
		t.Fatalf("got kind=%q", kind)
	}
}

func TestRunTimeWindowForScheduledRun(t *testing.T) {
	now := time.Date(2026, 4, 11, 5, 7, 0, 0, time.UTC)
	cmd := RunCmd{Scheduled: true, At: "05:00"}

	since, until, recapDate, kind, err := cmd.timeWindow(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSince := time.Date(2026, 4, 10, 5, 0, 0, 0, time.UTC)
	wantUntil := time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC)
	if !since.Equal(wantSince) || !until.Equal(wantUntil) || !recapDate.Equal(wantUntil) {
		t.Fatalf("got since=%v until=%v recapDate=%v", since, until, recapDate)
	}
	if kind != summary.KindSchedule {
		t.Fatalf("got kind=%q", kind)
	}
}

func TestRunTimeWindowRejectsConflictingFlags(t *testing.T) {
	now := time.Date(2026, 4, 11, 15, 31, 0, 0, time.UTC)
	cmd := RunCmd{Day: "2026-04-10", Scheduled: true, At: "05:00"}

	_, _, _, _, err := cmd.timeWindow(now)
	if err == nil {
		t.Fatal("expected error")
	}
}
