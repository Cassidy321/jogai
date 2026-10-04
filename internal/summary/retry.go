package summary

import (
	"context"
	"time"
)

// Each attempt gets its own timeout so one hung call (laptop asleep
// mid-response) cannot use up the time left for retries.
type Retry struct {
	Summarizer
	Backoff     []time.Duration
	CallTimeout time.Duration
	Sleep       func(ctx context.Context, d time.Duration) error
}

func NewRetry(s Summarizer) Retry {
	return Retry{
		Summarizer:  s,
		Backoff:     []time.Duration{30 * time.Second, 2 * time.Minute},
		CallTimeout: 10 * time.Minute,
		Sleep:       sleepCtx,
	}
}

func (r Retry) Generate(ctx context.Context, req Request) (*Summary, error) {
	for attempt := 0; ; attempt++ {
		s, err := r.attempt(ctx, req)
		if err == nil || KindOf(err) != KindTransient || attempt == len(r.Backoff) {
			return s, err
		}
		if err := r.Sleep(ctx, r.Backoff[attempt]); err != nil {
			return nil, err
		}
	}
}

func (r Retry) attempt(ctx context.Context, req Request) (*Summary, error) {
	ctx, cancel := context.WithTimeout(ctx, r.CallTimeout)
	defer cancel()
	return r.Summarizer.Generate(ctx, req)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
