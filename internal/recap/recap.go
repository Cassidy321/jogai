package recap

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/filter"
	"github.com/Cassidy321/jogai/internal/summary"
)

type Writer interface {
	// Asked before any summarizer call: a recap that will not be written,
	// like one the user edited by hand, must not cost a call per project.
	Check(s *summary.Summary) error
	Write(s *summary.Summary) error
}

type Archive interface {
	Day(since, until time.Time) ([]archive.Activity, error)
}

type Pipeline struct {
	Archive    Archive
	Summarizer summary.Summarizer
	Writer     Writer
	Warnings   []string
}

type Day struct {
	Span    devday.Span
	Summary *summary.Summary
	Refused int
	Err     error
}

const (
	outsideProjects = "Hors projet"
	refusedNote     = "_Résumé indisponible : le modèle a refusé de résumer cette partie de la journée._"
)

func (p *Pipeline) Run(ctx context.Context, spans []devday.Span) []Day {
	days := make([]Day, 0, len(spans))
	for _, span := range spans {
		s, refused, err := p.runDay(ctx, span)
		days = append(days, Day{Span: span, Summary: s, Refused: refused, Err: err})
		if summary.KindOf(err).NeedsUser() {
			break
		}
	}
	return days
}

// One failing project must not cost the others: the file is written with the
// sections that worked and the day is retried, as a whole, on the next run.
func (p *Pipeline) runDay(ctx context.Context, span devday.Span) (*summary.Summary, int, error) {
	activities, err := p.Archive.Day(span.Start, span.End)
	if err != nil {
		return nil, 0, fmt.Errorf("read archive: %w", err)
	}
	if len(activities) == 0 {
		return nil, 0, nil
	}
	s := &summary.Summary{Date: span.Start, WindowStart: span.Start, WindowEnd: span.End}
	if err := p.Writer.Check(s); err != nil {
		return nil, 0, fmt.Errorf("write output: %w", err)
	}
	var sections []string
	var failures []error
	refused := 0
	for _, a := range activities {
		heading := a.Project
		if heading == "" {
			heading = outsideProjects
		}
		body, err := p.summarize(ctx, span, a)
		if summary.KindOf(err).NeedsUser() {
			return nil, 0, fmt.Errorf("%s: %w", heading, err)
		}
		if err != nil && summary.KindOf(err) == summary.KindRefused {
			refused++
			body, err = refusedNote, nil
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", heading, err))
			continue
		}
		sections = append(sections, "## "+heading+"\n\n"+body)
	}
	failed := errors.Join(failures...)
	if len(sections) == 0 {
		return nil, 0, failed
	}
	warnings := p.Warnings
	if failed != nil {
		warnings = append(slices.Clone(warnings), fmt.Sprintf("%d project section(s) missing, retried on the next run", len(failures)))
	}
	s.Content, s.Warnings = strings.Join(sections, "\n\n"), warnings
	if err := p.Writer.Write(s); err != nil {
		return nil, 0, fmt.Errorf("write output: %w", err)
	}
	return s, refused, failed
}

func (p *Pipeline) summarize(ctx context.Context, span devday.Span, a archive.Activity) (string, error) {
	s, err := p.Summarizer.Generate(ctx, summary.Request{Day: span.Start, Project: a.Project, Sessions: filter.Reduce(a.Sessions)})
	if err != nil {
		return "", err
	}
	return flattenHeadings(strings.TrimSpace(s.Content)), nil
}

// jogai's "## project" headings are the file's structure (the search index
// splits recaps on them): headings written by the model become bold lines.
func flattenHeadings(body string) string {
	lines := strings.Split(body, "\n")
	inCode := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inCode = !inCode
			continue
		}
		if inCode || !strings.HasPrefix(l, "#") {
			continue
		}
		if t := strings.TrimSpace(strings.TrimLeft(l, "#")); t != "" {
			lines[i] = "**" + t + "**"
		}
	}
	return strings.Join(lines, "\n")
}
