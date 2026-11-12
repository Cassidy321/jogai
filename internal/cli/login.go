package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/Cassidy321/jogai/internal/claudetoken"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/summary"
)

type LoginCmd struct{}

const loginAttempts = 3

type loginDeps struct {
	supported  bool
	terminal   bool
	setupToken func() error
	askToken   func() (string, error)
	verify     func(token string) error
	save       func(token string) (expires time.Time, err error)
	out        io.Writer
}

func (c *LoginCmd) Run() error {
	cfg, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return err
	}
	return runLogin(summarizerName(cfg), systemLogin())
}

func summarizerName(cfg *config.Config) string {
	if cfg != nil && cfg.Summarizer != "" {
		return cfg.Summarizer
	}
	return summary.NameClaude
}

func runLogin(summarizer string, d loginDeps) error {
	if summarizer == summary.NameCodex {
		fmt.Fprintln(d.out, "Your recaps are written by Codex, which has no long-lived token.")
		fmt.Fprintln(d.out, "If Codex is logged out, run: codex login")
		return nil
	}
	if !d.supported {
		return fmt.Errorf("jogai login needs the macOS Keychain to keep the token: %w", claudetoken.ErrUnsupported)
	}
	if !d.terminal {
		return errors.New("jogai login opens your browser and asks you to paste a token — run it in a terminal")
	}
	fmt.Fprintln(d.out, "jogai login — connect your recaps to Claude for one year")
	fmt.Fprintln(d.out)
	fmt.Fprintln(d.out, "  Your browser opens next. Approve the access, then copy the token")
	fmt.Fprintln(d.out, "  Claude prints. jogai keeps it in your macOS Keychain.")
	fmt.Fprintln(d.out)
	if err := d.setupToken(); err != nil {
		return fmt.Errorf("claude setup-token failed (%w) — it needs a Claude subscription; with an API key, run `claude auth login` instead", err)
	}
	fmt.Fprintln(d.out)

	token, err := acceptedToken(d)
	if err != nil {
		return err
	}
	expires, err := d.save(token)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.out, "  ✓ Recaps are connected to Claude until %s\n", expires.Format("2006-01-02"))
	fmt.Fprintf(d.out, "    The token is in your macOS Keychain, under %q.\n", claudetoken.Service)
	fmt.Fprintln(d.out, "    jogai will remind you a month before it expires.")
	if hasFailedDays() {
		fmt.Fprintln(d.out, "    The dev days that failed are recapped on the next run, or now with `jogai run`.")
	}
	return nil
}

// Only a token Claude accepts is kept: a truncated paste must fail here, in
// front of the user, not in the scheduled run.
func acceptedToken(d loginDeps) (string, error) {
	for attempt := 1; ; attempt++ {
		pasted, err := d.askToken()
		if err != nil {
			return "", err
		}
		token := strings.Join(strings.Fields(pasted), "")
		if token == "" {
			return "", errors.New("no token pasted — nothing was changed")
		}
		err = d.verify(token)
		if err == nil {
			return token, nil
		}
		if summary.KindOf(err) != summary.KindAuth {
			return "", fmt.Errorf("could not check the token, nothing was saved — run `jogai login` again: %w", err)
		}
		if attempt == loginAttempts {
			return "", fmt.Errorf("the token was refused %d times — nothing was saved", loginAttempts)
		}
		fmt.Fprintln(d.out, "  ✗ Claude refused this token — copy it again, in full")
	}
}

func hasFailedDays() bool {
	days, err := lastrun.LoadDays()
	if err != nil {
		return false
	}
	for _, d := range days {
		if d.NeedsRetry() {
			return true
		}
	}
	return false
}

func systemLogin() loginDeps {
	return loginDeps{
		supported:  claudetoken.Supported(),
		terminal:   isTerminal(os.Stdin) && isTerminal(os.Stdout),
		setupToken: claudeSetupToken,
		askToken:   askToken,
		verify: func(token string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			return summary.Claude{Token: token}.Ping(ctx)
		},
		save: func(token string) (time.Time, error) {
			if err := claudetoken.New(claudetoken.Attended).Save(token); err != nil {
				return time.Time{}, err
			}
			st, _ := claudetoken.LoadState()
			return st.ExpiresAt(), nil
		},
		out: os.Stdout,
	}
}

func claudeSetupToken() error {
	bin, err := summary.LookPath(summary.NameClaude)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "setup-token")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func askToken() (string, error) {
	var token string
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Paste the token").
			Description("Hidden as you paste.").
			EchoMode(huh.EchoModePassword).
			Value(&token),
	)).WithTheme(jogaiTheme()).Run()
	return token, err
}
