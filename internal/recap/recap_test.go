package recap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/summary"
)

type fakeArchive func(since, until time.Time) ([]archive.Activity, error)

func (f fakeArchive) Day(since, until time.Time) ([]archive.Activity, error) { return f(since, until) }

type mockWriter struct {
	written []*summary.Summary
}

func (m *mockWriter) Write(s *summary.Summary) error {
	m.written = append(m.written, s)
	return nil
}

type fakeSummarizer struct {
	fn func(req summary.Request) (*summary.Summary, error)
}

func (f fakeSummarizer) Name() string    { return "fake" }
func (f fakeSummarizer) CheckCLI() error { return nil }
func (f fakeSummarizer) Generate(_ context.Context, req summary.Request) (*summary.Summary, error) {
	return f.fn(req)
}

var day6 = devday.Span{
	Start: time.Date(2026, 4, 6, 5, 0, 0, 0, time.UTC),
	End:   time.Date(2026, 4, 7, 5, 0, 0, 0, time.UTC),
	Label: "2026-04-06",
}

func activity(project string) archive.Activity {
	return archive.Activity{Project: project, Sessions: []parser.Session{{ID: project, Messages: []parser.Message{{Role: "user", Content: "work on " + project}}}}}
}

func threeProjects(time.Time, time.Time) ([]archive.Activity, error) {
	return []archive.Activity{activity("dokaa"), activity("jogai"), activity("")}, nil
}

func body(req summary.Request) (*summary.Summary, error) {
	return &summary.Summary{Content: "summary of " + req.Sessions[0].Messages[0].Content}, nil
}

func TestRun_OneSectionPerProject(t *testing.T) {
	w := &mockWriter{}
	var requests []summary.Request
	p := &Pipeline{
		Archive: fakeArchive(threeProjects),
		Summarizer: fakeSummarizer{fn: func(req summary.Request) (*summary.Summary, error) {
			requests = append(requests, req)
			return body(req)
		}},
		Writer: w,
	}
	days := p.Run(context.Background(), []devday.Span{day6})
	if len(days) != 1 || days[0].Err != nil || days[0].Summary == nil {
		t.Fatalf("days = %+v", days)
	}
	want := "## dokaa\n\nsummary of work on dokaa\n\n## jogai\n\nsummary of work on jogai\n\n## Hors projet\n\nsummary of work on"
	if got := w.written[0].Content; got != want {
		t.Errorf("content =\n%q\nwant\n%q", got, want)
	}
	if len(requests) != 3 || requests[0].Project != "dokaa" || requests[2].Project != "" || !requests[0].Day.Equal(day6.Start) {
		t.Errorf("requests = %+v", requests)
	}
	if s := w.written[0]; !s.Date.Equal(day6.Start) || !s.WindowEnd.Equal(day6.End) {
		t.Errorf("window = %v → %v", s.Date, s.WindowEnd)
	}
}

func TestRun_FlattensHeadingsWrittenByTheModel(t *testing.T) {
	w := &mockWriter{}
	p := &Pipeline{
		Archive: fakeArchive(func(time.Time, time.Time) ([]archive.Activity, error) {
			return []archive.Activity{activity("jogai")}, nil
		}),
		Summarizer: fakeSummarizer{fn: func(summary.Request) (*summary.Summary, error) {
			return &summary.Summary{Content: "### Détails\n- point\n```sh\n# a shell comment\n```"}, nil
		}},
		Writer: w,
	}
	p.Run(context.Background(), []devday.Span{day6})
	got := w.written[0].Content
	if !strings.Contains(got, "**Détails**") || strings.Contains(got, "### Détails") || !strings.Contains(got, "# a shell comment") {
		t.Errorf("content =\n%s", got)
	}
}

func TestRun_RefusedProjectGetsANote(t *testing.T) {
	w := &mockWriter{}
	p := &Pipeline{
		Archive: fakeArchive(threeProjects),
		Summarizer: fakeSummarizer{fn: func(req summary.Request) (*summary.Summary, error) {
			if req.Project == "jogai" {
				return nil, &summary.Error{Kind: summary.KindRefused, Msg: "flagged"}
			}
			return body(req)
		}},
		Writer: w,
	}
	days := p.Run(context.Background(), []devday.Span{day6})
	if days[0].Err != nil || !strings.Contains(w.written[0].Content, "## jogai\n\n"+refusedNote) {
		t.Errorf("day = %+v, content =\n%s", days[0], w.written[0].Content)
	}
}

func TestRun_FailingProjectKeepsTheOthers(t *testing.T) {
	w := &mockWriter{}
	p := &Pipeline{
		Archive: fakeArchive(threeProjects),
		Summarizer: fakeSummarizer{fn: func(req summary.Request) (*summary.Summary, error) {
			if req.Project == "dokaa" {
				return nil, errors.New("socket closed")
			}
			return body(req)
		}},
		Writer: w,
	}
	days := p.Run(context.Background(), []devday.Span{day6})
	d := days[0]
	if d.Summary == nil || d.Err == nil || !strings.Contains(d.Err.Error(), "dokaa") {
		t.Fatalf("day = %+v", d)
	}
	content := w.written[0].Content
	if strings.Contains(content, "## dokaa") || !strings.Contains(content, "## jogai") {
		t.Errorf("content =\n%s", content)
	}
	if !strings.Contains(strings.Join(w.written[0].Warnings, "\n"), "missing") {
		t.Errorf("warnings = %v", w.written[0].Warnings)
	}
}

func TestRun_EveryProjectFailing(t *testing.T) {
	w := &mockWriter{}
	p := &Pipeline{
		Archive:    fakeArchive(threeProjects),
		Summarizer: fakeSummarizer{fn: func(summary.Request) (*summary.Summary, error) { return nil, errors.New("down") }},
		Writer:     w,
	}
	days := p.Run(context.Background(), []devday.Span{day6})
	if days[0].Summary != nil || days[0].Err == nil || len(w.written) != 0 {
		t.Errorf("day = %+v, written %d", days[0], len(w.written))
	}
}

func TestRun_EmptyDay(t *testing.T) {
	p := &Pipeline{
		Archive: fakeArchive(func(time.Time, time.Time) ([]archive.Activity, error) { return nil, nil }),
		Summarizer: fakeSummarizer{fn: func(summary.Request) (*summary.Summary, error) {
			t.Fatal("no summary without activity")
			return nil, nil
		}},
		Writer: &mockWriter{},
	}
	days := p.Run(context.Background(), []devday.Span{day6})
	if days[0].Summary != nil || days[0].Err != nil {
		t.Errorf("day = %+v", days[0])
	}
}
