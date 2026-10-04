package recap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/summary"
)

type mockParser struct {
	sessions []parser.Session
	err      error
	since    time.Time
}

func (m *mockParser) Name() string { return "mock" }
func (m *mockParser) Detect() bool { return true }
func (m *mockParser) Sessions(since time.Time) ([]parser.Session, error) {
	m.since = since
	return m.sessions, m.err
}

type mockWriter struct {
	written []*summary.Summary
}

func (m *mockWriter) Write(s *summary.Summary) error {
	m.written = append(m.written, s)
	return nil
}

type fakeSummarizer struct {
	fn func(ctx context.Context, day time.Time, sessions []parser.Session) (*summary.Summary, error)
}

func (f fakeSummarizer) Name() string    { return "fake" }
func (f fakeSummarizer) CheckCLI() error { return nil }
func (f fakeSummarizer) Generate(ctx context.Context, req summary.Request) (*summary.Summary, error) {
	return f.fn(ctx, req.Day, req.Sessions)
}

func at(day, hour int) time.Time { return time.Date(2026, 4, day, hour, 0, 0, 0, time.UTC) }

func span(day int) devday.Span {
	return devday.Span{Start: at(day, 5), End: at(day+1, 5), Label: at(day, 5).Format(devday.LabelFormat)}
}

func twoDaysOfWork() []parser.Session {
	return []parser.Session{{
		ID: "s1", Project: "jogai",
		Messages: []parser.Message{
			{Role: "user", Content: "day 6 work", Timestamp: at(6, 10)},
			{Role: "user", Content: "day 7 work", Timestamp: at(7, 10)},
		},
	}}
}

func TestPipelineRun_WritesEachDay(t *testing.T) {
	mp := &mockParser{sessions: twoDaysOfWork()}
	w := &mockWriter{}
	var gotDays []time.Time
	p := &Pipeline{
		Parser: mp,
		Summarizer: fakeSummarizer{fn: func(_ context.Context, day time.Time, s []parser.Session) (*summary.Summary, error) {
			gotDays = append(gotDays, day)
			return &summary.Summary{Content: s[0].Messages[0].Content}, nil
		}},
		Writer: w,
	}

	days, err := p.Run(context.Background(), []devday.Span{span(6), span(7)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(days) != 2 || days[0].Summary == nil || days[1].Summary == nil {
		t.Fatalf("got %+v, want two written days", days)
	}
	if !mp.since.Equal(at(6, 5)) {
		t.Errorf("sources should be parsed once from the oldest day, since = %v", mp.since)
	}
	if len(w.written) != 2 || w.written[0].Content != "day 6 work" || w.written[1].Content != "day 7 work" {
		t.Fatalf("written = %+v", w.written)
	}
	second := w.written[1]
	if !second.Date.Equal(at(7, 5)) || !second.WindowStart.Equal(at(7, 5)) || !second.WindowEnd.Equal(at(8, 5)) {
		t.Errorf("day 7 window = %v [%v, %v)", second.Date, second.WindowStart, second.WindowEnd)
	}
	if !gotDays[0].Equal(at(6, 5)) {
		t.Errorf("summarizer should receive the dev day start, got %v", gotDays[0])
	}
}

func TestPipelineRun_EmptyDayIsNotSummarized(t *testing.T) {
	p := &Pipeline{
		Parser: &mockParser{},
		Summarizer: fakeSummarizer{fn: func(context.Context, time.Time, []parser.Session) (*summary.Summary, error) {
			t.Fatal("summarizer should not be called without sessions")
			return nil, nil
		}},
		Writer: &mockWriter{},
	}
	days, err := p.Run(context.Background(), []devday.Span{span(6)})
	if err != nil || len(days) != 1 || days[0].Summary != nil || days[0].Err != nil {
		t.Fatalf("Run = (%+v, %v), want one empty day", days, err)
	}
}

func TestPipelineRun_FailingDayDoesNotBlockOthers(t *testing.T) {
	boom := errors.New("boom")
	w := &mockWriter{}
	p := &Pipeline{
		Parser: &mockParser{sessions: twoDaysOfWork()},
		Summarizer: fakeSummarizer{fn: func(_ context.Context, day time.Time, _ []parser.Session) (*summary.Summary, error) {
			if day.Equal(at(6, 5)) {
				return nil, boom
			}
			return &summary.Summary{Content: "ok"}, nil
		}},
		Writer: w,
	}
	days, err := p.Run(context.Background(), []devday.Span{span(6), span(7)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(days[0].Err, boom) {
		t.Errorf("day 6 error = %v, want boom", days[0].Err)
	}
	if days[1].Err != nil || days[1].Summary == nil || len(w.written) != 1 {
		t.Errorf("day 7 should still be written, got %+v (written %d)", days[1], len(w.written))
	}
}

func TestPipelineRun_KeepsOnlyMessagesInsideTheDay(t *testing.T) {
	sessions := []parser.Session{{
		ID: "s1",
		Messages: []parser.Message{
			{Role: "user", Content: "before", Timestamp: at(7, 4)},
			{Role: "assistant", Content: "after", Timestamp: at(7, 6)},
		},
	}}
	var got []parser.Session
	p := &Pipeline{
		Parser: &mockParser{sessions: sessions},
		Summarizer: fakeSummarizer{fn: func(_ context.Context, _ time.Time, s []parser.Session) (*summary.Summary, error) {
			got = s
			return &summary.Summary{}, nil
		}},
		Writer: &mockWriter{},
	}
	if _, err := p.Run(context.Background(), []devday.Span{span(6)}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Messages) != 1 || got[0].Messages[0].Content != "before" {
		t.Fatalf("summarized = %+v, want only the 04:00 message", got)
	}
	if !got[0].StartedAt.Equal(at(7, 4)) || !got[0].EndedAt.Equal(at(7, 4)) {
		t.Errorf("session bounds = [%v, %v], want the kept message", got[0].StartedAt, got[0].EndedAt)
	}
}

func TestPipelineRun_ParseErrorFailsTheRun(t *testing.T) {
	p := &Pipeline{Parser: &mockParser{err: errors.New("every source failed")}, Writer: &mockWriter{}}
	if _, err := p.Run(context.Background(), []devday.Span{span(6)}); err == nil {
		t.Fatal("expected the parse error")
	}
}
