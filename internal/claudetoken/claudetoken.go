package claudetoken

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

const (
	service      = "jogai"
	account      = "claude-token"
	exitNotFound = 44
	stateFile    = "claude_token.json"

	lifetime   = 365 * 24 * time.Hour
	warnBefore = 30 * 24 * time.Hour
)

var ErrUnsupported = errors.New("the macOS Keychain is not available on this system")

type runner func(stdin string, args ...string) (stdout string, exit int, err error)

// The Keychain is reached through /usr/bin/security, never directly: macOS
// ties an entry to the program that wrote it, and jogai's binary changes with
// every upgrade, so the scheduled run would stop on a permission dialog nobody
// is there to answer.
type Store struct {
	run runner
	now func() time.Time
}

func Supported() bool { return runtime.GOOS == "darwin" }

func New() Store {
	run := runner(func(string, ...string) (string, int, error) { return "", 0, ErrUnsupported })
	if Supported() {
		run = security
	}
	return Store{run: run, now: time.Now}
}

func security(stdin string, args ...string) (string, int, error) {
	cmd := exec.Command("/usr/bin/security", args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode(), errors.New(strings.TrimSpace(stderr.String()))
	}
	return string(out), 0, err
}

// The token goes through stdin: arguments are visible to every process. It is
// read back because the scheduled run only ever reads: a token that cannot be
// read must fail here, in front of the user, not at night.
func (s Store) Save(token string) error {
	line := fmt.Sprintf("add-generic-password -U -s %s -a %s -X %s\n", service, account, hex.EncodeToString([]byte(token)))
	if _, _, err := s.run(line, "-i"); err != nil {
		return fmt.Errorf("save the token in the Keychain: %w", err)
	}
	kept, err := s.Load()
	if err != nil {
		return err
	}
	if kept != token {
		return errors.New("save the token in the Keychain: it was not kept")
	}
	if err := writeState(State{CreatedAt: s.now()}); err != nil {
		_, _ = s.Delete()
		return err
	}
	return nil
}

func (s Store) Load() (string, error) {
	out, exit, err := s.run("", "find-generic-password", "-s", service, "-a", account, "-w")
	switch {
	case errors.Is(err, ErrUnsupported):
		return "", nil
	case exit == exitNotFound:
		removeState()
		return "", nil
	case err != nil:
		return "", fmt.Errorf("read the token from the Keychain: %w", err)
	}
	return strings.TrimRight(out, "\n"), nil
}

func (s Store) Delete() (removed bool, err error) {
	_, exit, err := s.run("", "delete-generic-password", "-s", service, "-a", account)
	removeState()
	switch {
	case errors.Is(err, ErrUnsupported), exit == exitNotFound:
		return false, nil
	case err != nil:
		return false, fmt.Errorf("remove the token from the Keychain: %w", err)
	}
	return true, nil
}

// State is kept outside the Keychain so that status and the MCP server can
// tell when the token expires without reading the secret.
type State struct {
	CreatedAt time.Time `json:"created_at"`
}

func (s State) ExpiresAt() time.Time { return s.CreatedAt.Add(lifetime) }

func (s State) Expired(now time.Time) bool { return !now.Before(s.ExpiresAt()) }

func (s State) ExpiresSoon(now time.Time) bool {
	return !now.Before(s.ExpiresAt().Add(-warnBefore))
}

func LoadState() (State, bool) {
	path, err := statePath()
	if err != nil {
		return State{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, false
	}
	var s State
	if json.Unmarshal(data, &s) != nil || s.CreatedAt.IsZero() {
		return State{}, false
	}
	return s, true
}

func statePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFile), nil
}

func writeState(s State) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token state: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".claude-token-*.json")
	if err != nil {
		return fmt.Errorf("save token state: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("save token state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("save token state: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("save token state: %w", err)
	}
	return nil
}

func removeState() {
	if path, err := statePath(); err == nil {
		_ = os.Remove(path)
	}
}
