package scheduler

import (
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestNextRunAlreadyPassed(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.Local)
	got := nextRun(config.TimeOfDay{Hour: 9, Minute: 0}, now)
	want := time.Date(2026, 4, 7, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNextRunNotYetPassed(t *testing.T) {
	now := time.Date(2026, 4, 6, 8, 0, 0, 0, time.Local)
	got := nextRun(config.TimeOfDay{Hour: 9, Minute: 0}, now)
	want := time.Date(2026, 4, 6, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
