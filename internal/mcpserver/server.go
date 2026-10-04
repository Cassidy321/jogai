package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/devday"
)

const readBudget = 20_000

type Deps struct {
	Store          *archive.Store
	Refresh        func()
	ExcludeSession string
	Health         []string
	Version        string
}

// Claude Code defers tool descriptions (only names are listed until a tool is
// loaded): when to search must live in the server instructions, which are
// always in the system prompt.
const instructions = `jogai archives the user's interactive Claude Code and Codex sessions and their daily recaps, beyond Claude Code's 30-day transcript retention.

Search it whenever the user refers to past work — "comment on avait fait…", "la dernière fois", a decision, a bug already met, a project they come back to — before answering from memory or redoing the investigation.
- Keyword search: use exact identifiers, error messages and names; when results are thin, reformulate with synonyms and with both French and English words.
- Recaps (kind "recap") give the day and the vocabulary; then search messages with those exact terms.
- read(id) returns the whole exchange around a hit, starting at the question; follow the "more" id for the rest.
- The current session is never in the results.`

func New(d Deps) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "jogai", Version: d.Version}, &mcp.ServerOptions{Instructions: instructionsWith(d.Health)})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search",
		Description: "Search the user's past AI coding sessions and daily recaps by keywords. Returns short snippets with date, project, branch and an id for read.",
	}, d.search)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "read",
		Description: "Read the full exchange (or recap section) around an id returned by search.",
	}, d.read)
	return s
}

func instructionsWith(health []string) string {
	if len(health) == 0 {
		return instructions
	}
	return instructions + "\n\njogai needs the user's attention — tell them at the start of your reply:\n- " + strings.Join(health, "\n- ")
}

type searchInput struct {
	Query   string `json:"query" jsonschema:"keywords; exact identifiers, error messages or names work best"`
	Project string `json:"project,omitempty" jsonschema:"only this project (repository or folder name, e.g. jogai)"`
	Kind    string `json:"kind,omitempty" jsonschema:"recap for daily recaps only, message for session messages only"`
	Since   string `json:"since,omitempty" jsonschema:"YYYY-MM-DD, inclusive"`
	Until   string `json:"until,omitempty" jsonschema:"YYYY-MM-DD, exclusive"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of results, default 10, at most 50"`
}

type readInput struct {
	ID string `json:"id" jsonschema:"an id returned by search, or the 'more' id of a previous read"`
}

func (d Deps) search(_ context.Context, _ *mcp.CallToolRequest, in searchInput) (*mcp.CallToolResult, any, error) {
	d.Refresh()
	q := archive.Query{Text: in.Query, Project: in.Project, Kind: in.Kind, Limit: in.Limit, ExcludeSession: d.ExcludeSession}
	var err error
	if q.Since, err = parseDay(in.Since); err != nil {
		return toolError(err), nil, nil
	}
	if q.Until, err = parseDay(in.Until); err != nil {
		return toolError(err), nil, nil
	}
	hits, err := d.Store.Search(q)
	if err != nil {
		return nil, nil, err
	}
	if len(hits) == 0 {
		return textResult("No results. Try other keywords, synonyms, the other language (French/English), or fewer filters."), nil, nil
	}
	return textResult(archive.FormatHits(hits)), nil, nil
}

func (d Deps) read(_ context.Context, _ *mcp.CallToolRequest, in readInput) (*mcp.CallToolResult, any, error) {
	page, err := d.Store.Read(in.ID, readBudget)
	if err != nil {
		return toolError(err), nil, nil
	}
	return textResult(page.Text), nil, nil
}

func parseDay(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation(devday.LabelFormat, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q — expected YYYY-MM-DD", s)
	}
	return t, nil
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func toolError(err error) *mcp.CallToolResult {
	res := textResult(err.Error())
	res.IsError = true
	return res
}
