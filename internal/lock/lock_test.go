//go:build unix

package lock

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestTry_IsExclusiveUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x", "run.lock")
	release, err := Try(path)
	if err != nil {
		t.Fatalf("first Try: %v", err)
	}
	if _, err := Try(path); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Try = %v, want ErrBusy", err)
	}
	release()
	again, err := Try(path)
	if err != nil {
		t.Fatalf("Try after release: %v", err)
	}
	again()
}

func TestTryFor_WaitsForTheHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "refresh.lock")
	release, err := Try(path)
	if err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(300*time.Millisecond, release)
	got, err := TryFor(path, 5*time.Second)
	if err != nil {
		t.Fatalf("TryFor = %v, want the lock once released", err)
	}
	got()
	held, _ := Try(path)
	if _, err := TryFor(path, 300*time.Millisecond); !errors.Is(err, ErrBusy) {
		t.Errorf("TryFor on a held lock = %v, want ErrBusy after the wait", err)
	}
	held()
}
