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

func (c Claude) Generate(ctx context.Context, sessions []parser.Session) (*Summary, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions to summarize")
	}
	prompt, err := buildPrompt(sessions)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}
	resp, err := c.run(ctx, prompt)
	if err != nil {
		return nil, err
	}
	if resp.IsError {
		return nil, classifyClaudeError(resp.Result)
	}
	totalInput := resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens + resp.Usage.CacheReadInputTokens
	return &Summary{
		Date:     time.Now(),
		Content:  strings.TrimSpace(resp.Result),
		Sessions: len(sessions),
		Usage: Usage{
			InputTokens:  totalInput,
			OutputTokens: resp.Usage.OutputTokens,
			CostUSD:      resp.TotalCostUSD,
		},
	}, nil
}

type claudeResponse struct {
	Result       string  `json:"result"`
	IsError      bool    `json:"is_error"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func (c Claude) run(ctx context.Context, prompt string) (*claudeResponse, error) {
	if err := c.CheckCLI(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, NameClaude,
		"-p",
		"--output-format", "json",
		"--no-session-persistence",
	)
	// Neutral CWD so claude's CLAUDE.md walk-up doesn't cross the user's home and trigger macOS TCC prompts.
	cmd.Dir = os.TempDir()
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if len(out) > 0 {
			var resp claudeResponse
			if jsonErr := json.Unmarshal(out, &resp); jsonErr == nil {
				return &resp, nil
			}
		}
		return nil, fmt.Errorf("claude CLI failed — make sure you're logged in and have an active subscription\n  detail: %w\n  %s", err, stderr.String())
	}
	var resp claudeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("could not read claude response — try running 'claude -p' manually to check for issues\n  detail: %w", err)
	}
	return &resp, nil
}

func classifyClaudeError(result string) error {
	lower := strings.ToLower(result)
	switch {
	case strings.Contains(lower, "prompt is too long"):
		return fmt.Errorf("too many sessions to summarize at once — try a shorter time window with --day")
	case strings.Contains(lower, "rate limit"), strings.Contains(lower, "too many requests"):
		return fmt.Errorf("rate limit reached — wait a few minutes and try again")
	case strings.Contains(lower, "unauthorized"), strings.Contains(lower, "authentication"):
		return fmt.Errorf("authentication failed — check your Claude subscription or run 'claude auth'")
	default:
		return fmt.Errorf("summary generation failed: %s", result)
	}
}
