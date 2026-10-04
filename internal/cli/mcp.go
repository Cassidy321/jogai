package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
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
	// The first search must wait for the startup refresh: on a fresh install it
	// is the initial import, and searching before it ends finds nothing.
	var mu sync.Mutex
	refresh := func() bool {
		mu.Lock()
		defer mu.Unlock()
		res, err := env.refresh(0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "jogai: archive:", err)
		}
		return res.Busy
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
