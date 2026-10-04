package summary

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestClassify_RealFailures(t *testing.T) {
	tests := []struct {
		msg  string
		want Kind
	}{
		{"API Error: Stream idle timeout - partial response received", KindTransient},
		{"API Error: The socket connection was closed unexpectedly. For more information, pass `verbose: true`", KindTransient},
		{"API Error: Your computer went to sleep mid-response. The response above may be incomplete.", KindTransient},
		{"Failed to refresh OAuth token: another Claude Code process is refreshing it or exited mid-refresh.", KindTransient},
		{"API Error: Opus 5's safeguards flagged this message (https://www.anthropic.com/legal/aup).", KindRefused},
		{"Invalid API key · Please run /login", KindAuth},
		{"Not logged in · Please run /login", KindAuth},
		{"Prompt is too long", KindFatal},
	}
	for _, tt := range tests {
		if got := classify(tt.msg); got != tt.want {
			t.Errorf("classify(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestKindOf(t *testing.T) {
	wrapped := fmt.Errorf("generate summary: %w", &Error{Kind: KindRefused, Msg: "flagged"})
	if got := KindOf(wrapped); got != KindRefused {
		t.Errorf("KindOf(wrapped) = %v, want refused", got)
	}
	if got := KindOf(errors.New("plain")); got != KindFatal {
		t.Errorf("KindOf(plain) = %v, want fatal", got)
	}
	if got := KindOf(fmt.Errorf("x: %w", context.DeadlineExceeded)); got != KindTransient {
		t.Errorf("KindOf(deadline) = %v, want transient", got)
	}
}
