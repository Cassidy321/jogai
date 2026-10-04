package summary

import (
	"context"
	"slices"
	"testing"
	"time"
)

type scripted struct {
	errs      []error
	calls     int
	deadlines []bool
}

func (s *scripted) Name() string    { return "scripted" }
func (s *scripted) CheckCLI() error { return nil }
func (s *scripted) Generate(ctx context.Context, _ Request) (*Summary, error) {
	_, hasDeadline := ctx.Deadline()
	s.deadlines = append(s.deadlines, hasDeadline)
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, s.errs[i]
	}
	return &Summary{Content: "ok"}, nil
}

func testRetry(s Summarizer, slept *[]time.Duration) Retry {
	return Retry{
		Summarizer:  s,
		Backoff:     []time.Duration{time.Second, 2 * time.Second},
		CallTimeout: time.Minute,
		Sleep: func(_ context.Context, d time.Duration) error {
			*slept = append(*slept, d)
			return nil
		},
	}
}

func TestRetry_RetriesTransientFailures(t *testing.T) {
	transient := &Error{Kind: KindTransient, Msg: "socket closed"}
	s := &scripted{errs: []error{transient, transient}}
	var slept []time.Duration

	got, err := testRetry(s, &slept).Generate(context.Background(), Request{})
	if err != nil || got.Content != "ok" {
		t.Fatalf("Generate = (%v, %v), want ok", got, err)
	}
	if s.calls != 3 {
		t.Errorf("calls = %d, want 3", s.calls)
	}
	if !slices.Equal(slept, []time.Duration{time.Second, 2 * time.Second}) {
		t.Errorf("slept = %v", slept)
	}
	if !slices.Equal(s.deadlines, []bool{true, true, true}) {
		t.Errorf("every attempt needs its own timeout, deadlines = %v", s.deadlines)
	}
}

func TestRetry_GivesUpAfterTheLastAttempt(t *testing.T) {
	transient := &Error{Kind: KindTransient, Msg: "socket closed"}
	s := &scripted{errs: []error{transient, transient, transient}}
	var slept []time.Duration

	_, err := testRetry(s, &slept).Generate(context.Background(), Request{})
	if KindOf(err) != KindTransient || s.calls != 3 {
		t.Errorf("err = %v, calls = %d; want the transient error after 3 calls", err, s.calls)
	}
}

func TestRetry_DoesNotRetryOtherKinds(t *testing.T) {
	for _, kind := range []Kind{KindRefused, KindAuth, KindFatal} {
		s := &scripted{errs: []error{&Error{Kind: kind, Msg: "no"}}}
		var slept []time.Duration
		_, err := testRetry(s, &slept).Generate(context.Background(), Request{})
		if err == nil || s.calls != 1 || len(slept) != 0 {
			t.Errorf("kind %v: err = %v, calls = %d, slept = %v", kind, err, s.calls, slept)
		}
	}
}
