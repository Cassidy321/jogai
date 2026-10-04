package update

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

const (
	formula  = "cassidy321/tap/jogai"
	interval = 24 * time.Hour
)

func Daily(ctx context.Context) (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return false, err
	}
	brew, ok := brewFor(resolved)
	if !ok {
		return false, nil
	}
	dir, err := config.Dir()
	if err != nil {
		return false, err
	}
	stamp := filepath.Join(dir, "last_update_check")
	now := time.Now()
	if !due(stamp, now) {
		return false, nil
	}
	// Stamped before the upgrade: a failing brew must not be retried on every run.
	if err := touch(stamp, now); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, brew, "upgrade", formula).CombinedOutput()
	errPath := filepath.Join(dir, "last_update_error")
	if err != nil {
		failure := fmt.Errorf("brew upgrade %s: %w: %s", formula, err, lastLine(out))
		_ = os.WriteFile(errPath, []byte(failure.Error()), 0o644)
		return true, failure
	}
	_ = os.Remove(errPath)
	return true, nil
}

func LastError() string {
	dir, err := config.Dir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "last_update_error"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func HomebrewPrefix(path string) (string, bool) {
	prefix, _, ok := strings.Cut(path, "/Cellar/jogai/")
	return prefix, ok
}

func brewFor(path string) (string, bool) {
	prefix, ok := HomebrewPrefix(path)
	if !ok {
		return "", false
	}
	return filepath.Join(prefix, "bin", "brew"), true
}

func due(stamp string, now time.Time) bool {
	info, err := os.Stat(stamp)
	return err != nil || now.Sub(info.ModTime()) >= interval
}

func touch(path string, t time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chtimes(path, t, t)
}

func lastLine(out []byte) string {
	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	return string(lines[len(lines)-1])
}
