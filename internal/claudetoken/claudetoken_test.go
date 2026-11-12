package claudetoken

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeKeychain struct {
	secret  string
	present bool
	locked  bool
	dropAdd bool
	calls   [][]string
	stdin   []string
}

func (k *fakeKeychain) run(stdin string, args ...string) (string, int, error) {
	k.calls = append(k.calls, args)
	k.stdin = append(k.stdin, stdin)
	if k.locked {
		return "", 36, errors.New("User interaction is not allowed.")
	}
	switch args[0] {
	case "-i":
		if k.dropAdd {
			return "", 0, nil
		}
		fields := strings.Fields(stdin)
		raw, err := hex.DecodeString(fields[len(fields)-1])
		if err != nil {
			return "", 1, err
		}
		k.secret, k.present = string(raw), true
		return "", 0, nil
	case "find-generic-password":
		if !k.present {
			return "", exitNotFound, errors.New("The specified item could not be found in the keychain.")
		}
		return k.secret + "\n", 0, nil
	case "delete-generic-password":
		if !k.present {
			return "", exitNotFound, errors.New("The specified item could not be found in the keychain.")
		}
		k.present = false
		return "", 0, nil
	}
	return "", 1, errors.New("unexpected command")
}

var created = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

func newStore(t *testing.T, k *fakeKeychain) Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return Store{run: k.run, now: func() time.Time { return created }}
}

func TestSaveThenLoad(t *testing.T) {
	k := &fakeKeychain{}
	s := newStore(t, k)
	const token = "sk-ant-oat01-secret"
	if err := s.Save(token); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || got != token {
		t.Fatalf("Load = (%q, %v), want the saved token", got, err)
	}
	st, ok := LoadState()
	if !ok || !st.CreatedAt.Equal(created) {
		t.Errorf("LoadState = (%+v, %v), want created at %v", st, ok, created)
	}
	for _, args := range k.calls {
		if strings.Contains(strings.Join(args, " "), token) {
			t.Errorf("the token must never be passed as an argument: %q", args)
		}
	}
	if strings.Contains(strings.Join(k.stdin, " "), token) {
		t.Error("the token must be hex-encoded on stdin")
	}
}

func TestSave_ReplacesThePreviousToken(t *testing.T) {
	k := &fakeKeychain{secret: "old", present: true}
	s := newStore(t, k)
	if err := s.Save("new"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load(); got != "new" {
		t.Errorf("Load = %q, want new", got)
	}
}

func TestSave_NotKeptIsAnError(t *testing.T) {
	k := &fakeKeychain{dropAdd: true}
	s := newStore(t, k)
	if err := s.Save("token"); err == nil {
		t.Fatal("a token the Keychain did not keep must be reported")
	}
	if _, ok := LoadState(); ok {
		t.Error("no state without a stored token")
	}
}

func TestSave_UnwritableStateRemovesTheToken(t *testing.T) {
	k := &fakeKeychain{}
	s := newStore(t, k)
	home := os.Getenv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".config"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("token"); err == nil {
		t.Fatal("expected an error when the state cannot be written")
	}
	if k.present {
		t.Error("a token without its creation date must not stay in the Keychain")
	}
}

func TestLoad_MissingTokenDropsTheState(t *testing.T) {
	k := &fakeKeychain{}
	s := newStore(t, k)
	if err := writeState(State{CreatedAt: created}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil || got != "" {
		t.Fatalf("Load = (%q, %v), want no token and no error", got, err)
	}
	if _, ok := LoadState(); ok {
		t.Error("the state of a token that is gone must be removed")
	}
}

func TestLoad_UnreadableKeychainIsAnError(t *testing.T) {
	k := &fakeKeychain{present: true, secret: "token", locked: true}
	s := newStore(t, k)
	if err := writeState(State{CreatedAt: created}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("Load error = %v, want the Keychain failure", err)
	}
	if _, ok := LoadState(); !ok {
		t.Error("an unreadable Keychain says nothing about the token: keep the state")
	}
}

func TestDelete(t *testing.T) {
	k := &fakeKeychain{}
	s := newStore(t, k)
	if err := s.Save("token"); err != nil {
		t.Fatal(err)
	}
	removed, err := s.Delete()
	if err != nil || !removed {
		t.Fatalf("Delete = (%v, %v), want removed", removed, err)
	}
	if _, ok := LoadState(); ok || k.present {
		t.Error("the token and its state must both be gone")
	}
	if removed, err := s.Delete(); err != nil || removed {
		t.Errorf("second Delete = (%v, %v), want nothing to remove", removed, err)
	}
}

func TestUnsupportedSystem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := Store{run: func(string, ...string) (string, int, error) { return "", 0, ErrUnsupported }, now: time.Now}
	if got, err := s.Load(); err != nil || got != "" {
		t.Errorf("Load = (%q, %v), want no token and no error", got, err)
	}
	if removed, err := s.Delete(); err != nil || removed {
		t.Errorf("Delete = (%v, %v), want nothing", removed, err)
	}
	if err := s.Save("token"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Save error = %v, want ErrUnsupported", err)
	}
}

func TestStateExpiry(t *testing.T) {
	st := State{CreatedAt: created}
	expires := created.Add(365 * 24 * time.Hour)
	if !st.ExpiresAt().Equal(expires) {
		t.Fatalf("ExpiresAt = %v, want %v", st.ExpiresAt(), expires)
	}
	tests := []struct {
		name          string
		now           time.Time
		soon, expired bool
	}{
		{"fresh", created, false, false},
		{"31 days left", expires.Add(-31 * 24 * time.Hour), false, false},
		{"30 days left", expires.Add(-30 * 24 * time.Hour), true, false},
		{"one hour left", expires.Add(-time.Hour), true, false},
		{"expired", expires, true, true},
	}
	for _, tt := range tests {
		if got := st.ExpiresSoon(tt.now); got != tt.soon {
			t.Errorf("%s: ExpiresSoon = %v, want %v", tt.name, got, tt.soon)
		}
		if got := st.Expired(tt.now); got != tt.expired {
			t.Errorf("%s: Expired = %v, want %v", tt.name, got, tt.expired)
		}
	}
}

func TestLoadState_CorruptIsMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, err := statePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadState(); ok {
		t.Error("a corrupt state must read as no state")
	}
}
