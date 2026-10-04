package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
)

func connect(t *testing.T, d Deps) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := New(d).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func archived(t *testing.T) *archive.Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "-w-jogai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	session := `{"type":"user","uuid":"m1","sessionId":"s1","cwd":"/w/jogai","timestamp":"2026-10-01T10:00:00Z","message":{"role":"user","content":"caffeinate garde le Mac éveillé"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(session), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := archive.Open(filepath.Join(t.TempDir(), "jogai.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cc, err := parser.NewClaudeCode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Refresh([]parser.Source{cc}, &project.Resolver{Home: home}, "", 0); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestServer(t *testing.T) {
	refreshed := 0
	cs := connect(t, Deps{Store: archived(t), Refresh: func() bool { refreshed++; return false }, Health: []string{"the recap of 2026-10-02 failed"}, Version: "test"})
	ctx := context.Background()

	instructions := cs.InitializeResult().Instructions
	for _, want := range []string{"search", "read(id)", "the recap of 2026-10-02 failed"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("instructions missing %q:\n%s", want, instructions)
		}
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "search", Arguments: map[string]any{"query": "caffeinate"}})
	if err != nil || res.IsError || !strings.Contains(text(res), "id: m1") || refreshed != 1 {
		t.Fatalf("search = (%q, %v), refreshed %d", text(res), err, refreshed)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"id": "m1"}})
	if err != nil || res.IsError || !strings.Contains(text(res), "éveillé") {
		t.Fatalf("read = (%q, %v)", text(res), err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "search", Arguments: map[string]any{"query": "x", "since": "01/10/2026"}})
	if err != nil || !res.IsError {
		t.Errorf("a malformed date must be a tool error, got (%q, %v)", text(res), err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "search", Arguments: map[string]any{"query": "nothingmatcheshere"}})
	if err != nil || res.IsError || !strings.Contains(text(res), "No results") {
		t.Errorf("empty search = (%q, %v)", text(res), err)
	}
}

func TestServer_HealthyInstructionsStayShort(t *testing.T) {
	cs := connect(t, Deps{Store: archived(t), Refresh: func() bool { return false }, Version: "test"})
	if strings.Contains(cs.InitializeResult().Instructions, "attention") {
		t.Error("no health section expected when everything is fine")
	}
}

func TestServer_SaysWhenAnotherProcessIsRefreshing(t *testing.T) {
	cs := connect(t, Deps{Store: archived(t), Refresh: func() bool { return true }, Version: "test"})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "search", Arguments: map[string]any{"query": "caffeinate"}})
	if err != nil || !strings.Contains(text(res), "id: m1") || !strings.Contains(text(res), "may be missing") {
		t.Errorf("search during a foreign refresh = (%q, %v)", text(res), err)
	}
}
