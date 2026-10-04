package summary

import (
	"context"
	"errors"
	"strings"
)

// Kind decides what the caller does with a failed day: retry now with backoff
// (Transient), retry on the next run (Fatal, Auth), never retry (Refused: the
// same content would be refused again).
type Kind int

const (
	KindFatal Kind = iota
	KindTransient
	KindRefused
	KindAuth
)

func (k Kind) String() string {
	switch k {
	case KindFatal:
		return "fatal"
	case KindTransient:
		return "transient"
	case KindRefused:
		return "refused"
	case KindAuth:
		return "auth"
	}
	return "unknown"
}

type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return KindTransient
	}
	return KindFatal
}

// Patterns come from real daily.err.log entries. Transient ones are matched
// first: the OAuth refresh race mentions OAuth but the CLI is still logged in.
var (
	transientPatterns = []string{
		"stream idle timeout",
		"socket connection was closed",
		"went to sleep",
		"another claude code process is refreshing",
		"rate limit",
		"too many requests",
		"overloaded",
		"econnreset",
		"etimedout",
		"network error",
	}
	refusedPatterns = []string{
		"safeguards flagged",
		"anthropic.com/legal/aup",
		"usage policy",
	}
	authPatterns = []string{
		"not logged in",
		"please run /login",
		"invalid api key",
		"authentication",
		"unauthorized",
		"codex login",
	}
)

func classify(text string) Kind {
	lower := strings.ToLower(text)
	switch {
	case containsAny(lower, transientPatterns):
		return KindTransient
	case containsAny(lower, refusedPatterns):
		return KindRefused
	case containsAny(lower, authPatterns):
		return KindAuth
	default:
		return KindFatal
	}
}

func containsAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
