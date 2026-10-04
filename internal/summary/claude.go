package summary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/parser"
)

type Claude struct{}

func (Claude) Name() string { return NameClaude }

func (Claude) CheckCLI() error { return checkCLI(NameClaude) }

// The summarizer must not inherit the interactive setup: user hooks and
// plugins would fire on every call, MCP servers would spawn `jogai mcp`, the
// user's default model (opus[1m]) can get recaps refused by safeguards, and a
// persisted session would be recapped the next day.
var claudeArgs = []string{
	"-p",
	"--output-format", "json",
	"--no-session-persistence",
	"--model", "sonnet",
	"--tools", "",
	"--strict-mcp-config",
	"--setting-sources", "",
}

func (c Claude) Generate(ctx context.Context, day time.Time, sessions []parser.Session) (*Summary, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions to summarize")
	}
	prompt, err := buildPrompt(day, sessions)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}
	resp, err := c.run(ctx, prompt)
	if err != nil {
		return nil, err
	}
	if resp.IsError {
		return nil, &Error{Kind: classify(resp.Result), Msg: "claude: " + resp.Result}
	}
	return &Summary{Content: strings.TrimSpace(resp.Result)}, nil
}

type claudeResponse struct {
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
}

func (c Claude) run(ctx context.Context, prompt string) (*claudeResponse, error) {
	bin, err := LookPath(NameClaude)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, claudeArgs...)
	// Neutral CWD so claude's CLAUDE.md walk-up doesn't cross the user's home and trigger macOS TCC prompts.
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(prompt)
	// Children of a killed claude can keep stdout open; without a delay Output never returns.
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if ctx.Err() != nil {
		return nil, &Error{Kind: KindTransient, Msg: "claude CLI timed out"}
	}
	// API errors exit non-zero but still print the JSON result that names the cause.
	var resp claudeResponse
	if json.Unmarshal(out, &resp) == nil && (resp.IsError || resp.Result != "") {
		return &resp, nil
	}
	if runErr != nil {
		detail := strings.TrimSpace(stderr.String())
		return nil, &Error{Kind: classify(detail), Msg: fmt.Sprintf("claude CLI failed (%v): %s", runErr, detail)}
	}
	return nil, &Error{Kind: KindFatal, Msg: "could not read the claude response — try running 'claude -p' manually"}
}
