package recap

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Cassidy321/jogai/internal/devday"
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

type Day struct {
	Span    devday.Span
	Summary *summary.Summary
	Err     error
}

// One failing day must not block the others; the returned error is only for
// failures that hit every day.
func (p *Pipeline) Run(ctx context.Context, spans []devday.Span) ([]Day, error) {
	if len(spans) == 0 {
		return nil, nil
	}
	all, err := p.Parser.Sessions(spans[0].Start)
	if err != nil {
		return nil, fmt.Errorf("parse sessions: %w", err)
	}
	warnings := collectWarnings(p.Parser)

	days := make([]Day, 0, len(spans))
	for _, span := range spans {
		s, err := p.runDay(ctx, span, all, warnings)
		days = append(days, Day{Span: span, Summary: s, Err: err})
	}
	return days, nil
}

func (p *Pipeline) runDay(ctx context.Context, span devday.Span, all []parser.Session, warnings []string) (*summary.Summary, error) {
	sessions := sessionsIn(all, span.Start, span.End)
	if len(sessions) == 0 {
		return nil, nil
	}
	s, err := p.Summarizer.Generate(ctx, summary.Request{Day: span.Start, Sessions: filter.Reduce(sessions)})
	if err != nil {
		return nil, fmt.Errorf("generate summary: %w", err)
	}
	s.Date = span.Start
	s.WindowStart = span.Start
	s.WindowEnd = span.End
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

func sessionsIn(all []parser.Session, since, until time.Time) []parser.Session {
	var out []parser.Session
	for _, s := range all {
		var msgs []parser.Message
		for _, m := range s.Messages {
			if !m.Timestamp.Before(since) && m.Timestamp.Before(until) {
				msgs = append(msgs, m)
			}
		}
		if len(msgs) == 0 {
			continue
		}
		s.Messages = msgs
		s.StartedAt = msgs[0].Timestamp
		s.EndedAt = msgs[len(msgs)-1].Timestamp
		out = append(out, s)
	}
	slices.SortStableFunc(out, func(a, b parser.Session) int { return a.StartedAt.Compare(b.StartedAt) })
	return out
}
