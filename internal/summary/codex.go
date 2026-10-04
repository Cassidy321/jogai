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

func (c Codex) Generate(ctx context.Context, day time.Time, sessions []parser.Session) (*Summary, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions to summarize")
	}
	bin, err := LookPath(NameCodex)
	if err != nil {
		return nil, err
	}
	prompt, err := buildPrompt(day, sessions)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "jogai-codex-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	outPath := filepath.Join(tmpDir, "last_message.txt")

	cmd := exec.CommandContext(ctx, bin, codexArgs(outPath)...)
	cmd.Dir = tmpDir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, &Error{Kind: KindTransient, Msg: "codex CLI timed out"}
		}
		detail := strings.TrimSpace(stderr.String())
		return nil, &Error{Kind: classify(detail), Msg: fmt.Sprintf("codex CLI failed (%v): %s", err, detail)}
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read codex output: %w", err)
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		return nil, &Error{Kind: KindFatal, Msg: "codex returned an empty recap"}
	}
	return &Summary{Content: content}, nil
}

// --ephemeral: a persisted session would be ingested and recapped the next day.
func codexArgs(outPath string) []string {
	return []string{"exec", "--ephemeral", "--skip-git-repo-check", "-s", "read-only", "--output-last-message", outPath, "-"}
}
