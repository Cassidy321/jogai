package recap

import (
	"context"
	"fmt"
	"time"

	"github.com/Cassidy321/jogai/internal/filter"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/summary"
)

type Writer interface {
	Write(s *summary.Summary) error
}

type Pipeline struct {
	Parser     parser.Parser
	Summarizer summary.Summarizer
	Writer     Writer
}

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

	warnings := collectWarnings(p.Parser)

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
	s.Warnings = warnings

	if err := p.Writer.Write(s); err != nil {
		return nil, fmt.Errorf("write output: %w", err)
	}

	return s, nil
}

func collectWarnings(p parser.Parser) []string {
	if w, ok := p.(interface{ Warnings() []string }); ok {
		return w.Warnings()
	}
	return nil
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
