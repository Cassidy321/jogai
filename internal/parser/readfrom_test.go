package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	claudeUser      = `{"type":"user","uuid":"u1","sessionId":"s1","cwd":"/w/jogai","timestamp":"2026-04-05T10:00:00Z","message":{"role":"user","content":"add tests"}}`
	claudeAssistant = `{"type":"assistant","uuid":"u2","sessionId":"s1","cwd":"/w/jogai","timestamp":"2026-04-05T10:00:05Z","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}`
)

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeReadFrom_ResumesAfterTheLastCompleteLine(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	path := filepath.Join(cc.baseDir, "s1.jsonl")
	appendFile(t, path, claudeUser+"\n"+claudeAssistant[:40])

	records, cur, err := cc.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 1 || records[0].ID != "u1" {
		t.Fatalf("first read = (%+v, %v)", records, err)
	}
	if cur.Offset != int64(len(claudeUser)+1) {
		t.Errorf("offset = %d, want the end of the first line", cur.Offset)
	}

	appendFile(t, path, claudeAssistant[40:]+"\n")
	records, _, err = cc.ReadFrom(path, cur)
	if err != nil || len(records) != 1 || records[0].ID != "u2" {
		t.Fatalf("resumed read = (%+v, %v), want the completed line only", records, err)
	}
}

func TestClaudeReadFrom_FallsBackToOffsetWithoutUUID(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	path := filepath.Join(cc.baseDir, "s1.jsonl")
	appendFile(t, path, `{"type":"user","sessionId":"s1","timestamp":"2026-04-05T10:00:00Z","message":{"role":"user","content":"old format"}}`+"\n")
	records, _, err := cc.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 1 || records[0].ID != "s1:0" {
		t.Fatalf("records = (%+v, %v), want id s1:0", records, err)
	}
}

func TestClaudeFiles_SkipsSubagents(t *testing.T) {
	cc := &ClaudeCode{baseDir: t.TempDir()}
	main := filepath.Join(cc.baseDir, "-w-jogai", "s1.jsonl")
	sub := filepath.Join(cc.baseDir, "-w-jogai", "s1", "subagents", "agent-1.jsonl")
	for _, p := range []string{main, sub} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		appendFile(t, p, claudeUser+"\n")
	}
	files, err := cc.Files()
	if err != nil || len(files) != 1 || files[0] != main {
		t.Errorf("Files = (%v, %v), want only %s", files, err, main)
	}
}

func TestCodexReadFrom_CarriesSessionStateAcrossReads(t *testing.T) {
	cx := &Codex{baseDir: t.TempDir()}
	path := filepath.Join(cx.baseDir, "2026", "10", "01", "rollout-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	appendFile(t, path, strings.Join([]string{
		`{"timestamp":"2026-10-01T10:00:00Z","type":"session_meta","payload":{"id":"cx1","cwd":"/w/dokaa","originator":"codex-tui"}}`,
		`{"timestamp":"2026-10-01T10:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"regarde la PR"}}`,
	}, "\n")+"\n")

	records, cur, err := cx.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 1 || records[0].SessionID != "cx1" || records[0].Cwd != "/w/dokaa" {
		t.Fatalf("first read = (%+v, %v)", records, err)
	}

	appendFile(t, path, strings.Join([]string{
		`{"timestamp":"2026-10-01T10:05:00Z","type":"turn_context","payload":{"cwd":"/w/dokaa/apps/forms"}}`,
		`{"timestamp":"2026-10-01T10:05:01Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Je regarde."}]}}`,
	}, "\n")+"\n")
	records, _, err = cx.ReadFrom(path, cur)
	if err != nil || len(records) != 1 {
		t.Fatalf("resumed read = (%+v, %v)", records, err)
	}
	if r := records[0]; r.SessionID != "cx1" || r.Cwd != "/w/dokaa/apps/forms" || r.Role != "assistant" {
		t.Errorf("resumed record = %+v, want session cx1 in the turn's cwd", r)
	}
}

func TestCodexReadFrom_SkipsAutomatedSessionsForGood(t *testing.T) {
	cx := &Codex{baseDir: t.TempDir()}
	path := filepath.Join(cx.baseDir, "rollout-sdk.jsonl")
	src, err := os.ReadFile(filepath.Join("testdata", "codex_sdk_cwd.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	appendFile(t, path, string(src))

	records, cur, err := cx.ReadFrom(path, Cursor{})
	if err != nil || len(records) != 0 || !cur.Skip {
		t.Errorf("ReadFrom = (%+v, %+v, %v), want nothing and a skip cursor", records, cur, err)
	}
}

func TestCodexFiles_MissingDirIsEmpty(t *testing.T) {
	cx := &Codex{baseDir: filepath.Join(t.TempDir(), "missing")}
	if files, err := cx.Files(); err != nil || len(files) != 0 {
		t.Errorf("Files = (%v, %v), want none", files, err)
	}
}
