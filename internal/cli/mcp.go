package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/health"
	"github.com/Cassidy321/jogai/internal/mcpserver"
)

type MCPCmd struct{}

// stdout carries the MCP protocol: nothing else may print to it here.
func (c *MCPCmd) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, _ := config.Load()
	env, err := openArchive(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = env.store.Close() }()
	refresh := func() {
		if _, err := env.refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "jogai: archive:", err)
		}
	}
	go refresh()

	stats, _ := env.store.Stats()
	warning, _ := env.store.FormatWarning()
	server := mcpserver.New(mcpserver.Deps{
		Store:          env.store,
		Refresh:        refresh,
		ExcludeSession: os.Getenv("CLAUDE_CODE_SESSION_ID"),
		Health:         health.Warnings(health.Inputs{Now: time.Now(), LastIngest: stats.LastIngest, FormatWarning: warning}),
		Version:        version,
	})
	return server.Run(ctx, &mcp.StdioTransport{})
}
