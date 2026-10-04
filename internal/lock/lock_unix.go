//go:build unix

package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// The lock file is never deleted: a process that removed it would let the
// next one lock a fresh file while another still holds the old one.
func Try(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	return func() { _ = f.Close() }, nil
}
