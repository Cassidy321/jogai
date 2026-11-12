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
		{"Login expired · Please run /login", KindAuth},
		{"Failed to authenticate: OAuth session expired and could not be refreshed", KindAuth},
		{"Failed to authenticate. API Error: 401 OAuth access token is invalid.", KindAuth},
		{"ERROR: Your access token could not be refreshed. Please log out and sign in again.", KindAuth},
		{"ERROR: unexpected status 401 Unauthorized: Missing bearer or basic authentication in header, url: https://api.openai.com/v1/responses", KindAuth},
		{"ERROR: The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account.", KindConfig},
		{"Prompt is too long", KindFatal},
	}
	for _, tt := range tests {
		if got := classify(tt.msg); got != tt.want {
			t.Errorf("classify(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestKindNeedsUser(t *testing.T) {
	for kind, want := range map[Kind]bool{
		KindFatal: false, KindTransient: false, KindRefused: false, KindAuth: true, KindConfig: true,
	} {
		if got := kind.NeedsUser(); got != want {
			t.Errorf("%v.NeedsUser() = %v, want %v", kind, got, want)
		}
	}
	if KindOf(nil).NeedsUser() {
		t.Error("no error needs nobody")
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
