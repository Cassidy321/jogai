package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
)

var (
	testNow   = time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	fiveAM    = config.TimeOfDay{Hour: 5}
	noHistory = map[string]lastrun.Day{}
	noRecap   = func(string) bool { return false }
)

func TestRunSpans_DefaultsToRecentDaysWithoutRecap(t *testing.T) {
	spans, err := (&RunCmd{}).spans(testNow, fiveAM, noHistory, noRecap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spans) != catchUpDays {
		t.Fatalf("got %d spans, want %d", len(spans), catchUpDays)
	}
	last := spans[len(spans)-1]
	if last.Label != "2026-04-17" || !last.Start.Equal(time.Date(2026, 4, 17, 5, 0, 0, 0, time.UTC)) {
		t.Errorf("last span = %+v, want dev day 2026-04-17", last)
	}
}

func TestRunSpans_ForSpecificDay(t *testing.T) {
	spans, err := (&RunCmd{Day: "2026-04-15"}).spans(testNow, fiveAM, noHistory, noRecap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantStart := time.Date(2026, 4, 15, 5, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 4, 16, 5, 0, 0, 0, time.UTC)
	if len(spans) != 1 || spans[0].Label != "2026-04-15" || !spans[0].Start.Equal(wantStart) || !spans[0].End.Equal(wantEnd) {
		t.Fatalf("spans = %+v, want dev day 2026-04-15 [%v, %v)", spans, wantStart, wantEnd)
	}
}

func TestRunSpans_CalendarDayBoundary(t *testing.T) {
	spans, err := (&RunCmd{Day: "2026-04-15"}).spans(testNow, config.TimeOfDay{}, noHistory, noRecap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spans[0].Start.Equal(time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)) || !spans[0].End.Equal(time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("span = %+v, want the calendar day", spans[0])
	}
}

func TestRunSpans_SpecificDayIgnoresHistory(t *testing.T) {
	history := map[string]lastrun.Day{"2026-04-15": {Status: lastrun.StatusOK}}
	spans, err := (&RunCmd{Day: "2026-04-15"}).spans(testNow, fiveAM, history, func(string) bool { return true })
	if err != nil || len(spans) != 1 {
		t.Fatalf("--day must regenerate an already recapped day, got (%+v, %v)", spans, err)
	}
}

func TestRunSpans_RejectsInvalidDateFormat(t *testing.T) {
	_, err := (&RunCmd{Day: "04/15/2026"}).spans(testNow, fiveAM, noHistory, noRecap)
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("err = %v, want a format error", err)
	}
}

func TestRunSpans_RejectsInProgressDay(t *testing.T) {
	// dev day 2026-04-18 runs from 04-18 05:00 to 04-19 05:00; at 14:00 it's still in progress.
	_, err := (&RunCmd{Day: "2026-04-18"}).spans(testNow, fiveAM, noHistory, noRecap)
	if err == nil || !strings.Contains(err.Error(), "not yet complete") {
		t.Fatalf("err = %v, want an incomplete-day error", err)
	}
}

func TestRunSpans_RejectsFutureDay(t *testing.T) {
	if _, err := (&RunCmd{Day: "2026-04-20"}).spans(testNow, fiveAM, noHistory, noRecap); err == nil {
		t.Fatal("expected error for a future day")
	}
}

func TestRunSpans_AcceptsLegacyFlagsButIgnoresThem(t *testing.T) {
	spans, err := (&RunCmd{Scheduled: true, At: "09:00"}).spans(testNow, fiveAM, noHistory, noRecap)
	if err != nil || spans[len(spans)-1].Label != "2026-04-17" {
		t.Fatalf("legacy flags changed the selection: (%+v, %v)", spans, err)
	}
}

func TestPendingSpans(t *testing.T) {
	candidates := devday.Recent(testNow, fiveAM, 4)
	history := map[string]lastrun.Day{
		"2026-04-14": {Status: lastrun.StatusOK},
		"2026-04-15": {Status: lastrun.StatusError},
		"2026-04-16": {Status: lastrun.StatusRefused},
	}
	exists := func(label string) bool { return label == "2026-04-17" }

	got := pendingSpans(candidates, history, exists)
	if len(got) != 1 || got[0].Label != "2026-04-15" {
		t.Errorf("pending = %+v, want only the failed 2026-04-15", got)
	}

	got = pendingSpans(candidates, map[string]lastrun.Day{}, noRecap)
	if len(got) != 4 {
		t.Errorf("without history or files every day is pending, got %d", len(got))
	}
}

func TestBuildActiveParsers_DefaultsToClaude(t *testing.T) {
	cfg := &config.Config{}
	parsers, err := activeSources(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsers) != 1 {
		t.Fatalf("got %d parsers, want 1", len(parsers))
	}
	if parsers[0].Name() != "claude-code" {
		t.Errorf("default should be claude-code, got %q", parsers[0].Name())
	}
}

func TestBuildActiveParsers_HonorsConfig(t *testing.T) {
	cfg := &config.Config{Sources: []string{"codex"}}
	parsers, err := activeSources(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsers) != 1 {
		t.Fatalf("got %d parsers, want 1", len(parsers))
	}
	if parsers[0].Name() != "codex" {
		t.Errorf("got %q, want codex", parsers[0].Name())
	}
}

func TestBuildActiveParsers_UnknownSource(t *testing.T) {
	cfg := &config.Config{Sources: []string{"cursor"}}
	_, err := activeSources(cfg)
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestSelectSummarizer_DefaultsToClaude(t *testing.T) {
	s := selectSummarizer("")
	if s.Name() != "claude" {
		t.Errorf("default summarizer = %q, want claude", s.Name())
	}
}

func TestSelectSummarizer_Codex(t *testing.T) {
	s := selectSummarizer("codex")
	if s.Name() != "codex" {
		t.Errorf("got %q, want codex", s.Name())
	}
}

func TestRun_MultiSourcePartialFailureWritesWarningHeader(t *testing.T) {
	installClaudeStub(t)
	t.Setenv("XPC_SERVICE_NAME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeClaudeSession(t, home, time.Date(2026, 4, 21, 10, 0, 0, 0, time.Local))

	// Codex day dir: a file inside, then chmod 0o000, so reading it fails.
	cxDay := filepath.Join(home, ".codex", "sessions", "2026", "04", "21")
	if err := os.MkdirAll(cxDay, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cxDay, "rollout-stub.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cxDay, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cxDay, 0o755) })

	outDir := filepath.Join(home, "recaps")
	cfgDir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{"output_dir":"` + outDir + `","day_end":"00:00","sources":["claude-code","codex"],"summarizer":"claude"}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (&RunCmd{Day: "2026-04-21"}).Run(); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "2026-04-21.md"))
	if err != nil {
		t.Fatalf("expected recap file: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "> ⚠ some sessions could not be read") {
		t.Errorf("expected the read failure in a warning blockquote:\n%s", body)
	}
	if !strings.Contains(body, "## Hors projet") || !strings.Contains(body, "stub content") {
		t.Errorf("expected the recap body:\n%s", body)
	}
}

func installClaudeStub(t *testing.T) (callsFile string) {
	t.Helper()
	bindir := t.TempDir()
	stub := `#!/bin/sh
/bin/cat > /dev/null
echo call >> "$CALLS_FILE"
echo '{"result":"stub content","is_error":false}'
`
	if err := os.WriteFile(filepath.Join(bindir, "claude"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	callsFile = filepath.Join(bindir, "calls")
	t.Setenv("CALLS_FILE", callsFile)
	t.Setenv("PATH", bindir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return callsFile
}

func writeClaudeSession(t *testing.T, home string, at ...time.Time) {
	t.Helper()
	var lines []string
	for i, ts := range at {
		stamp := ts.UTC().Format(time.RFC3339)
		id := strconv.Itoa(i)
		lines = append(lines,
			`{"type":"user","uuid":"u`+id+`","sessionId":"s1","cwd":"/tmp/test","timestamp":"`+stamp+`","message":{"role":"user","content":"work"}}`,
			`{"type":"assistant","uuid":"a`+id+`","sessionId":"s1","cwd":"/tmp/test","timestamp":"`+stamp+`","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`,
		)
	}
	dir := filepath.Join(home, ".claude", "projects", "-tmp-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "s1.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRunConfig(t *testing.T, home string) (outDir string) {
	t.Helper()
	outDir = filepath.Join(home, "recaps")
	dir := filepath.Join(home, ".config", "jogai")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"output_dir":"` + outDir + `","day_end":"00:00","sources":["claude-code"],"summarizer":"claude"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return outDir
}

func noonDaysAgo(n int) time.Time {
	d := time.Now().AddDate(0, 0, -n)
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, time.Local)
}

func TestRun_CatchesUpMissedDays(t *testing.T) {
	calls := installClaudeStub(t)
	t.Setenv("XPC_SERVICE_NAME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeClaudeSession(t, home, noonDaysAgo(3), noonDaysAgo(1))
	outDir := writeRunConfig(t, home)

	if err := (&RunCmd{}).Run(); err != nil {
		t.Fatalf("first run: %v", err)
	}
	for _, n := range []int{3, 1} {
		label := noonDaysAgo(n).Format(devday.LabelFormat)
		if _, err := os.Stat(filepath.Join(outDir, label+".md")); err != nil {
			t.Errorf("expected a recap for %s: %v", label, err)
		}
	}
	days, err := lastrun.LoadDays()
	if err != nil {
		t.Fatal(err)
	}
	if got := days[noonDaysAgo(1).Format(devday.LabelFormat)].Status; got != lastrun.StatusOK {
		t.Errorf("yesterday status = %s, want ok", got)
	}
	if got := days[noonDaysAgo(2).Format(devday.LabelFormat)].Status; got != lastrun.StatusEmpty {
		t.Errorf("day without sessions status = %s, want empty", got)
	}

	if err := (&RunCmd{}).Run(); err != nil {
		t.Fatalf("second run: %v", err)
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	recap, err := os.ReadFile(filepath.Join(outDir, noonDaysAgo(1).Format(devday.LabelFormat)+".md"))
	if err != nil || !strings.Contains(string(recap), "## Hors projet") {
		t.Errorf("recap = (%q, %v), want a Hors projet section for sessions started in a temp dir", recap, err)
	}
	if n := strings.Count(string(data), "call"); n != 2 {
		t.Errorf("summarizer called %d times over two runs, want 2", n)
	}

	if n := archivedMessages(t); n != 4 {
		t.Errorf("archive holds %d messages, want the 4 session lines", n)
	}
}

func archivedMessages(t *testing.T) int {
	t.Helper()
	path, err := archive.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	store, err := archive.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	st, err := store.Stats()
	if err != nil {
		t.Fatal(err)
	}
	return st.Messages
}

func TestRun_KeepsARecapEditedByHand(t *testing.T) {
	installClaudeStub(t)
	t.Setenv("XPC_SERVICE_NAME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeClaudeSession(t, home, noonDaysAgo(1))
	outDir := writeRunConfig(t, home)
	if err := (&RunCmd{}).Run(); err != nil {
		t.Fatalf("first run: %v", err)
	}

	label := noonDaysAgo(1).Format(devday.LabelFormat)
	path := filepath.Join(outDir, label+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := string(data) + "\nmy own note\n"
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	days, err := lastrun.LoadDays()
	if err != nil {
		t.Fatal(err)
	}
	d := days[label]
	d.Status = lastrun.StatusError
	days[label] = d
	if err := lastrun.SaveDays(days); err != nil {
		t.Fatal(err)
	}

	if err := (&RunCmd{}).Run(); err != nil {
		t.Fatalf("catch-up run: %v", err)
	}
	if kept, _ := os.ReadFile(path); string(kept) != edited {
		t.Errorf("the edited recap was replaced:\n%s", kept)
	}
	days, _ = lastrun.LoadDays()
	if days[label].Status != lastrun.StatusOK {
		t.Errorf("status = %s, want ok (the edited recap is kept)", days[label].Status)
	}
}
