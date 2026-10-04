//go:build unix

package lock

import (
	"errors"
	"path/filepath"
	"testing"
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
