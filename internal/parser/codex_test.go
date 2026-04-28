package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexParseSessionFile_Happy(t *testing.T) {
	s, err := parseCodexSessionFile(filepath.Join("testdata", "codex_happy.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Tool != "codex" {
		t.Errorf("Tool = %q, want codex", s.Tool)
	}
	if s.Project != "dokaa" {
		t.Errorf("Project = %q, want dokaa", s.Project)
	}
	if len(s.Messages) != 4 {
		t.Fatalf("got %d messages, want 4", len(s.Messages))
	}
	if s.Messages[0].Role != "user" || s.Messages[0].Content != "regarde la PR 2132" {
		t.Errorf("msg[0] = %+v", s.Messages[0])
	}
	if s.Messages[1].Role != "assistant" || s.Messages[1].Content != "Je regarde la PR." {
		t.Errorf("msg[1] = %+v", s.Messages[1])
	}
	wantStart := time.Date(2026, 4, 14, 15, 16, 46, 0, time.UTC)
	if !s.StartedAt.Equal(wantStart) {
		t.Errorf("StartedAt = %v, want %v", s.StartedAt, wantStart)
	}
}

func TestCodexParseSessionFile_SDKCwd(t *testing.T) {
	s, err := parseCodexSessionFile(filepath.Join("testdata", "codex_sdk_cwd.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected session")
	}
	if s.Project != "unknown" {
		t.Errorf("Project = %q, want unknown", s.Project)
	}
}

func TestCodexParseSessionFile_Corrupt(t *testing.T) {
	s, err := parseCodexSessionFile(filepath.Join("testdata", "codex_corrupt.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.Project != "proj" {
		t.Errorf("Project = %q, want proj", s.Project)
	}
	if len(s.Messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(s.Messages))
	}
}

func TestCodexParseSessionFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parseCodexSessionFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Errorf("expected nil session for empty file, got %+v", s)
	}
}

func TestCodexDetect(t *testing.T) {
	dir := t.TempDir()
	c := &Codex{baseDir: filepath.Join(dir, "missing")}
	if c.Detect() {
		t.Error("Detect returned true for missing dir")
	}
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	c2 := &Codex{baseDir: real}
	if !c2.Detect() {
		t.Error("Detect returned false for existing dir")
	}
}

func TestCodexSessions_WalksDayDirs(t *testing.T) {
	dir := t.TempDir()
	day := filepath.Join(dir, "2026", "04", "14")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("testdata", "codex_happy.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(day, "rollout-1.jsonl"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Codex{baseDir: dir}
	sessions, err := c.Sessions(time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Tool != "codex" {
		t.Errorf("Tool = %q, want codex", sessions[0].Tool)
	}
}
