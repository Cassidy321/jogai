package recap

import (
	"context"
	"fmt"
	"time"

	"github.com/Cassidy321/jogai/internal/filter"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/summary"
)

// Summarizer generates a summary from filtered sessions.
type Summarizer interface {
	Generate(ctx context.Context, sessions []parser.Session) (*summary.Summary, error)
}

// Writer writes a summary to persistent storage.
type Writer interface {
	Write(s *summary.Summary) error
}

// SummarizerFunc adapts a function to the Summarizer interface.
type SummarizerFunc func(ctx context.Context, sessions []parser.Session) (*summary.Summary, error)

func (f SummarizerFunc) Generate(ctx context.Context, sessions []parser.Session) (*summary.Summary, error) {
	return f(ctx, sessions)
}

// Pipeline orchestrates the recap generation flow.
type Pipeline struct {
	Parser     parser.Parser
	Summarizer Summarizer
	Writer     Writer
}

// Run executes the full recap pipeline for sessions in the given time window.
func (p *Pipeline) Run(ctx context.Context, since, until, recapDate time.Time) (*summary.Summary, error) {
	allSessions, err := p.Parser.Sessions(since)
	if err != nil {
		return nil, fmt.Errorf("parse sessions: %w", err)
	}

	var sessions []parser.Session
	for _, s := range allSessions {
		msgs := messagesBefore(s.Messages, until)
		if len(msgs) == 0 {
			continue
		}
		s.Messages = msgs
		s.StartedAt = msgs[0].Timestamp
		s.EndedAt = msgs[len(msgs)-1].Timestamp
		sessions = append(sessions, s)
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	filtered := filter.Reduce(sessions)

	s, err := p.Summarizer.Generate(ctx, filtered)
	if err != nil {
		return nil, fmt.Errorf("generate summary: %w", err)
	}

	s.Date = recapDate
	s.WindowStart = since
	s.WindowEnd = until

	if err := p.Writer.Write(s); err != nil {
		return nil, fmt.Errorf("write output: %w", err)
	}

	return s, nil
}

func messagesBefore(messages []parser.Message, until time.Time) []parser.Message {
	if len(messages) == 0 || messages[len(messages)-1].Timestamp.Before(until) {
		return messages
	}
	var filtered []parser.Message
	for _, m := range messages {
		if m.Timestamp.Before(until) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}
