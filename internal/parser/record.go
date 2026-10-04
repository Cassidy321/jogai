package parser

import "time"

// Raw text plus the flags Clean reads: the archive stores records unfiltered so
// the cleaning rules can still change after Claude Code deletes the transcripts.
type Record struct {
	ID        string
	SessionID string
	Source    string
	Role      string
	Text      string
	Answers   string
	Title     string
	Timestamp time.Time
	Cwd       string
	GitBranch string
	IsMeta    bool
	IsCompact bool
	Origin    string
}

// Codex writes the session id and cwd only near the top of a file, so a read
// resuming mid-file needs them carried over.
type Cursor struct {
	Offset    int64
	SessionID string
	Cwd       string
	Skip      bool
}

type Source interface {
	Parser
	Files() ([]string, error)
	ReadFrom(path string, cur Cursor) ([]Record, Cursor, error)
}
