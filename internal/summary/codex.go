package summary

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/parser"
)

type Codex struct{}

func (Codex) Name() string { return NameCodex }

func (Codex) CheckCLI() error { return checkCLI(NameCodex) }

func (c Codex) Generate(ctx context.Context, sessions []parser.Session) (*Summary, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions to summarize")
	}
	if err := c.CheckCLI(); err != nil {
		return nil, err
	}
	prompt, err := buildPrompt(sessions)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "jogai-codex-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	outPath := filepath.Join(tmpDir, "last_message.txt")

	cmd := exec.CommandContext(ctx, NameCodex,
		"exec",
		"--skip-git-repo-check",
		"-s", "read-only",
		"--output-last-message", outPath,
		"-",
	)
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, classifyCodexError(stderr.String(), err)
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read codex output: %w", err)
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		return nil, fmt.Errorf("codex returned an empty recap")
	}
	return &Summary{
		Date:     time.Now(),
		Content:  content,
		Sessions: len(sessions),
		// Usage left zero — codex exec does not expose cost/tokens in a stable way.
	}, nil
}

func classifyCodexError(stderr string, err error) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "too many requests"):
		return fmt.Errorf("rate limit reached on Codex — wait a few minutes and try again")
	case strings.Contains(lower, "unauthorized"), strings.Contains(lower, "auth"):
		return fmt.Errorf("codex authentication failed — run 'codex login'")
	case strings.Contains(lower, "quota"), strings.Contains(lower, "credit"):
		return fmt.Errorf("codex quota or credits exhausted — check your account")
	}
	return fmt.Errorf("codex CLI failed — make sure you're logged in and have access\n  detail: %w\n  %s", err, stderr)
}
