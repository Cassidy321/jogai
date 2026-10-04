//go:build unix

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLock_IsExclusive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	release, err := AcquireLock()
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	if _, err := AcquireLock(); !errors.Is(err, ErrLocked) {
		t.Fatalf("second AcquireLock = %v, want ErrLocked", err)
	}
	release()

	again, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock after release: %v", err)
	}
	again()
}

func TestAcquireLock_IgnoresLegacyPIDFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// PID 1 is always alive.
	if err := os.WriteFile(filepath.Join(dir, "run.lock"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	release, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock with a legacy PID file: %v", err)
	}
	release()
}
