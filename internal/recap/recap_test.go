package recap

import (
	"context"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/summary"
)

type mockParser struct {
	sessions []parser.Session
}

func (m *mockParser) Name() string                                     { return "mock" }
func (m *mockParser) Detect() bool                                     { return true }
func (m *mockParser) Sessions(_ time.Time) ([]parser.Session, error)   { return m.sessions, nil }

type mockWriter struct {
	written *summary.Summary
}

func (m *mockWriter) Write(s *summary.Summary) error {
	m.written = s
	return nil
}

func TestPipelineRun(t *testing.T) {
	sessions := []parser.Session{
		{
			ID:        "s1",
			Tool:      "claude-code",
			StartedAt: time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
			EndedAt:   time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC),
			Project:   "jogai",
			Messages: []parser.Message{
				{Role: "user", Content: "add tests", Timestamp: time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)},
				{Role: "assistant", Content: "Done.", Timestamp: time.Date(2026, 4, 6, 10, 1, 0, 0, time.UTC)},
			},
		},
	}

	w := &mockWriter{}
	p := &Pipeline{
		Parser: &mockParser{sessions: sessions},
		Summarizer: SummarizerFunc(func(_ context.Context, s []parser.Session) (*summary.Summary, error) {
			return &summary.Summary{
				Date:     time.Now(),
				Content:  "Test recap",
				Sessions: len(s),
			}, nil
		}),
		Writer: w,
	}

	since := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)

	s, err := p.Run(context.Background(), since, until, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected summary, got nil")
	}
	if s.Sessions != 1 {
		t.Errorf("expected 1 session, got %d", s.Sessions)
	}
	if w.written == nil {
		t.Fatal("writer was not called")
	}
	if w.written.Content != "Test recap" {
		t.Errorf("expected 'Test recap', got %q", w.written.Content)
	}
}

func TestPipelineNoSessions(t *testing.T) {
	p := &Pipeline{
		Parser:     &mockParser{sessions: nil},
		Summarizer: SummarizerFunc(func(_ context.Context, _ []parser.Session) (*summary.Summary, error) {
			t.Fatal("summarizer should not be called when no sessions")
			return nil, nil
		}),
		Writer: &mockWriter{},
	}

	since := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)

	s, err := p.Run(context.Background(), since, until, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Errorf("expected nil summary for no sessions, got %+v", s)
	}
}

func TestPipelineFiltersUntil(t *testing.T) {
	sessions := []parser.Session{
		{
			ID:        "s1",
			StartedAt: time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
			Messages:  []parser.Message{{Role: "user", Content: "old", Timestamp: time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)}},
		},
		{
			ID:        "s2",
			StartedAt: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC),
			Messages:  []parser.Message{{Role: "user", Content: "future", Timestamp: time.Date(2026, 4, 7, 10, 0, 0, 0, time.UTC)}},
		},
	}

	var summarized int
	p := &Pipeline{
		Parser: &mockParser{sessions: sessions},
		Summarizer: SummarizerFunc(func(_ context.Context, s []parser.Session) (*summary.Summary, error) {
			summarized = len(s)
			return &summary.Summary{Sessions: len(s)}, nil
		}),
		Writer: &mockWriter{},
	}

	since := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)

	_, err := p.Run(context.Background(), since, until, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summarized != 1 {
		t.Errorf("expected 1 session after until filter, got %d", summarized)
	}
}
