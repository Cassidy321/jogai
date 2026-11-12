package summary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Codex struct{}

func (Codex) Name() string { return NameCodex }

func (Codex) CheckCLI() error { return checkCLI(NameCodex) }

const ignoreUserConfig = "--ignore-user-config"

var errNoIgnoreUserConfig = errors.New("codex does not know " + ignoreUserConfig)

func (c Codex) Generate(ctx context.Context, req Request) (*Summary, error) {
	if len(req.Sessions) == 0 {
		return nil, fmt.Errorf("no sessions to summarize")
	}
	bin, err := LookPath(NameCodex)
	if err != nil {
		return nil, err
	}
	prompt, err := buildPrompt(req)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}
	content, err := c.exec(ctx, bin, prompt, true)
	if errors.Is(err, errNoIgnoreUserConfig) {
		content, err = c.exec(ctx, bin, prompt, false)
	}
	if err != nil {
		return nil, err
	}
	return &Summary{Content: content}, nil
}

func (c Codex) exec(ctx context.Context, bin, prompt string, isolated bool) (string, error) {
	tmpDir, err := os.MkdirTemp("", "jogai-codex-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	outPath := filepath.Join(tmpDir, "last_message.txt")

	cmd := exec.CommandContext(ctx, bin, codexArgs(outPath, isolated)...)
	cmd.Dir = tmpDir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", &Error{Kind: KindTransient, Msg: "codex CLI timed out"}
		}
		detail := strings.TrimSpace(stderr.String())
		if isolated && strings.Contains(detail, "unexpected argument '"+ignoreUserConfig+"'") {
			return "", errNoIgnoreUserConfig
		}
		return "", &Error{Kind: classify(detail), Msg: fmt.Sprintf("codex CLI failed (%v): %s", err, detail)}
	}

	body, err := os.ReadFile(outPath)
	if err != nil {
		return "", fmt.Errorf("read codex output: %w", err)
	}
	content := strings.TrimSpace(string(body))
	if content == "" {
		return "", &Error{Kind: KindFatal, Msg: "codex returned an empty recap"}
	}
	return content, nil
}

// --ephemeral: a persisted session would be ingested and recapped the next day.
// --ignore-user-config: config.toml is the interactive setup. Its pinned model
// fails every recap once Codex retires it, and its MCP servers would start on
// each call. A Codex that predates the flag runs without it.
func codexArgs(outPath string, isolated bool) []string {
	args := []string{"exec"}
	if isolated {
		args = append(args, ignoreUserConfig)
	}
	return append(args, "--ephemeral", "--skip-git-repo-check", "-s", "read-only", "--output-last-message", outPath, "-")
}
