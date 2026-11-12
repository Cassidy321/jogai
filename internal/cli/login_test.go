package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/claudetoken"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/summary"
)

type loginScript struct {
	pasted   []string
	verdicts []error
	setupErr error
	saveErr  error

	setupRuns int
	asked     int
	verified  []string
	saved     []string
	out       bytes.Buffer
}

var loginExpiry = time.Date(2027, 11, 12, 10, 0, 0, 0, time.UTC)

func (s *loginScript) deps() loginDeps {
	return loginDeps{
		supported:  true,
		terminal:   true,
		setupToken: func() error { s.setupRuns++; return s.setupErr },
		askToken: func() (string, error) {
			s.asked++
			if s.asked > len(s.pasted) {
				return "", errors.New("asked more often than scripted")
			}
			return s.pasted[s.asked-1], nil
		},
		verify: func(token string) error {
			s.verified = append(s.verified, token)
			return s.verdicts[len(s.verified)-1]
		},
		save: func(token string) (time.Time, error) {
			s.saved = append(s.saved, token)
			return loginExpiry, s.saveErr
		},
		out: &s.out,
	}
}

var refused = &summary.Error{Kind: summary.KindAuth, Msg: "claude: 401"}

func TestLogin_SavesAnAcceptedToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := &loginScript{pasted: []string{"sk-ant-oat01-abc"}, verdicts: []error{nil}}
	if err := runLogin(summary.NameClaude, s.deps()); err != nil {
		t.Fatal(err)
	}
	if len(s.saved) != 1 || s.saved[0] != "sk-ant-oat01-abc" {
		t.Errorf("saved = %q", s.saved)
	}
	out := s.out.String()
	if !strings.Contains(out, "until 2027-11-12") || strings.Contains(out, "jogai run") {
		t.Errorf("output =\n%s", out)
	}
}

func TestLogin_MentionsTheDaysThatFailed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := lastrun.SaveDays(map[string]lastrun.Day{"2026-11-10": {Status: lastrun.StatusError, Kind: lastrun.KindAuth}}); err != nil {
		t.Fatal(err)
	}
	s := &loginScript{pasted: []string{"token"}, verdicts: []error{nil}}
	if err := runLogin(summary.NameClaude, s.deps()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.out.String(), "jogai run") {
		t.Errorf("output should say the failed days are caught up:\n%s", s.out.String())
	}
}

func TestLogin_JoinsATokenPastedOnSeveralLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := &loginScript{pasted: []string{"  sk-ant-oat01-abc\n  def \r\n"}, verdicts: []error{nil}}
	if err := runLogin(summary.NameClaude, s.deps()); err != nil {
		t.Fatal(err)
	}
	if len(s.saved) != 1 || s.saved[0] != "sk-ant-oat01-abcdef" {
		t.Errorf("saved = %q, want the token without whitespace", s.saved)
	}
}

func TestLogin_AsksAgainWhenClaudeRefuses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := &loginScript{pasted: []string{"truncated", "whole"}, verdicts: []error{refused, nil}}
	if err := runLogin(summary.NameClaude, s.deps()); err != nil {
		t.Fatal(err)
	}
	if len(s.saved) != 1 || s.saved[0] != "whole" || s.setupRuns != 1 {
		t.Errorf("saved = %q after %d setup run(s), want only the accepted token", s.saved, s.setupRuns)
	}
	if !strings.Contains(s.out.String(), "refused") {
		t.Errorf("output should say the token was refused:\n%s", s.out.String())
	}
}

func TestLogin_SavesNothing(t *testing.T) {
	tests := []struct {
		name   string
		script *loginScript
		want   string
	}{
		{"refused every time", &loginScript{pasted: []string{"a", "b", "c"}, verdicts: []error{refused, refused, refused}}, "refused 3 times"},
		{"check impossible", &loginScript{pasted: []string{"a"}, verdicts: []error{&summary.Error{Kind: summary.KindTransient, Msg: "network error"}}}, "network error"},
		{"nothing pasted", &loginScript{pasted: []string{" \n"}}, "no token pasted"},
		{"setup-token failed", &loginScript{setupErr: errors.New("exit status 1")}, "claude auth login"},
	}
	for _, tt := range tests {
		t.Setenv("HOME", t.TempDir())
		err := runLogin(summary.NameClaude, tt.script.deps())
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want it to mention %q", tt.name, err, tt.want)
		}
		if len(tt.script.saved) != 0 {
			t.Errorf("%s: saved = %q, want nothing", tt.name, tt.script.saved)
		}
	}
}

func TestLogin_NeedsATerminal(t *testing.T) {
	s := &loginScript{}
	d := s.deps()
	d.terminal = false
	if err := runLogin(summary.NameClaude, d); err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Errorf("err = %v, want it to ask for a terminal", err)
	}
	if s.setupRuns != 0 {
		t.Error("setup-token must not start without a terminal")
	}
}

func TestLogin_NeedsTheKeychain(t *testing.T) {
	s := &loginScript{}
	d := s.deps()
	d.supported = false
	if err := runLogin(summary.NameClaude, d); !errors.Is(err, claudetoken.ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
	if s.setupRuns != 0 {
		t.Error("setup-token must not start when the token cannot be kept")
	}
}

func TestLogin_CodexHasNoToken(t *testing.T) {
	s := &loginScript{}
	d := s.deps()
	d.terminal = false
	if err := runLogin(summary.NameCodex, d); err != nil {
		t.Fatal(err)
	}
	if s.setupRuns != 0 || !strings.Contains(s.out.String(), "codex login") {
		t.Errorf("setup runs = %d, output =\n%s", s.setupRuns, s.out.String())
	}
}

func TestLogin_SaveFailureIsReported(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := &loginScript{pasted: []string{"token"}, verdicts: []error{nil}, saveErr: errors.New("keychain locked")}
	err := runLogin(summary.NameClaude, s.deps())
	if err == nil || !strings.Contains(err.Error(), "keychain locked") || strings.Contains(s.out.String(), "connected") {
		t.Errorf("err = %v, output =\n%s", err, s.out.String())
	}
}
