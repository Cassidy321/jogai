package summary

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/parser"
)

func TestBuildPrompt(t *testing.T) {
	sessions := []parser.Session{
		{
			ID:        "s1",
			Tool:      "claude-code",
			StartedAt: time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
			EndedAt:   time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
			Project:   "jogai",
			Messages: []parser.Message{
				{Role: "user", Content: "add a login page"},
				{Role: "assistant", Content: "I'll create a login page for you."},
			},
		},
	}

	prompt, err := buildPrompt(sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "1 AI coding session") {
		t.Error("prompt should mention session count")
	}
	if !strings.Contains(prompt, "daily recap") {
		t.Error("prompt should mention daily recap")
	}
	if !strings.Contains(prompt, "Do not include frontmatter or a document title/heading") {
		t.Error("prompt should forbid a generated title")
	}
	if !strings.Contains(prompt, "jogai") {
		t.Error("prompt should include project name")
	}
	if !strings.Contains(prompt, "add a login page") {
		t.Error("prompt should include user message")
	}
	if !strings.Contains(prompt, "I'll create a login page") {
		t.Error("prompt should include assistant message")
	}
}

func TestBuildPromptMultipleSessions(t *testing.T) {
	sessions := []parser.Session{
		{
			ID:      "s1",
			Project: "jogai",
			Messages: []parser.Message{
				{Role: "user", Content: "first session"},
			},
		},
		{
			ID:      "s2",
			Project: "socadb",
			Messages: []parser.Message{
				{Role: "user", Content: "second session"},
			},
		},
	}

	prompt, err := buildPrompt(sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "2 AI coding session") {
		t.Error("prompt should mention 2 sessions")
	}
	if !strings.Contains(prompt, "jogai") {
		t.Error("prompt should contain first project name")
	}
	if !strings.Contains(prompt, "socadb") {
		t.Error("prompt should contain second project name")
	}
	if !strings.Contains(prompt, "<sessions>") {
		t.Error("prompt should wrap sessions in tags")
	}
	if !strings.Contains(prompt, "</sessions>") {
		t.Error("prompt should close sessions tag")
	}
}

func TestGenerateNoSessions(t *testing.T) {
	_, err := Generate(context.Background(), nil)
	if err == nil {
		t.Error("expected error for empty sessions")
	}
}
