package parser

import (
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

type fakeParser struct {
	name     string
	sessions []Session
	err      error
}

func (f *fakeParser) Name() string { return f.name }
func (f *fakeParser) Detect() bool { return true }
func (f *fakeParser) Sessions(since time.Time) ([]Session, error) {
	return f.sessions, f.err
}

func TestMultiParser_HappyPath(t *testing.T) {
	a := &fakeParser{name: "a", sessions: []Session{{ID: "s1", Tool: "a", StartedAt: time.Unix(10, 0)}}}
	b := &fakeParser{name: "b", sessions: []Session{{ID: "s2", Tool: "b", StartedAt: time.Unix(20, 0)}}}
	m := &MultiParser{Parsers: []Parser{a, b}}
	got, err := m.Sessions(time.Unix(0, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2", len(got))
	}
	sort.Slice(got, func(i, j int) bool { return got[i].StartedAt.Before(got[j].StartedAt) })
	if got[0].ID != "s1" || got[1].ID != "s2" {
		t.Errorf("order wrong: %v", got)
	}
	if len(m.Warnings()) != 0 {
		t.Errorf("Warnings = %v, want empty", m.Warnings())
	}
}

func TestMultiParser_PartialFailure(t *testing.T) {
	a := &fakeParser{name: "a", sessions: []Session{{ID: "s1", Tool: "a", StartedAt: time.Unix(10, 0)}}}
	b := &fakeParser{name: "b", err: errors.New("permission denied")}
	m := &MultiParser{Parsers: []Parser{a, b}}
	got, err := m.Sessions(time.Unix(0, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Errorf("got %v, want only s1", got)
	}
	warns := m.Warnings()
	if len(warns) != 1 {
		t.Fatalf("Warnings = %v, want 1", warns)
	}
	if warns[0] == "" || !strings.Contains(warns[0], "b") || !strings.Contains(warns[0], "permission denied") {
		t.Errorf("warning = %q", warns[0])
	}
}

func TestMultiParser_AllFailed(t *testing.T) {
	a := &fakeParser{name: "a", err: errors.New("boom")}
	b := &fakeParser{name: "b", err: errors.New("crash")}
	m := &MultiParser{Parsers: []Parser{a, b}}
	_, err := m.Sessions(time.Unix(0, 0))
	if err == nil {
		t.Fatal("expected an error when every source fails")
	}
	if !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "crash") {
		t.Errorf("error should list every source failure, got %v", err)
	}
}
