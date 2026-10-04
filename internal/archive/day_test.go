package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func (f *fixture) appendSession(t *testing.T, file, cwd string, lines ...string) {
	t.Helper()
	path := filepath.Join(f.home, ".claude", "projects", "-"+strings.ReplaceAll(strings.Trim(cwd, "/"), "/", "-"), file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fh.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}
}

func msg(uuid, session, cwd, ts, text string, extra string) string {
	return `{"type":"user","uuid":"` + uuid + `","sessionId":"` + session + `","cwd":"` + cwd + `","timestamp":"` + ts + `"` + extra + `,"message":{"role":"user","content":"` + text + `"}}`
}

func TestDay_GroupsCleanedActivityByProject(t *testing.T) {
	f := newFixture(t)
	s, _ := openTemp(t)
	f.appendSession(t, "j.jsonl", "/w/jogai",
		msg("j1", "sj", "/w/jogai", "2026-10-01T09:00:00Z", "jogai one", ""),
		msg("j2", "sj", "/w/jogai", "2026-10-01T09:05:00Z", "jogai two", ""),
		msg("j3", "sj", "/w/jogai", "2026-10-01T09:06:00Z", "Base directory for this skill", `,"isMeta":true`),
	)
	f.appendSession(t, "d.jsonl", "/w/dokaa",
		msg("d1", "sd", "/w/dokaa", "2026-10-01T10:00:00Z", "dokaa one", ""),
		msg("d2", "sd", "/w/dokaa", "2026-10-01T10:01:00Z", "dokaa two", ""),
		msg("d3", "sd", "/w/dokaa", "2026-10-01T10:02:00Z", "dokaa three", ""),
		msg("d4", "sd", "/w/dokaa", "2026-10-02T10:00:00Z", "next day", ""),
	)
	f.appendSession(t, "h.jsonl", f.home, msg("h1", "sh", f.home, "2026-10-01T11:00:00Z", "a quick question", ""))
	if _, err := s.Ingest(f.sources, f.res); err != nil {
		t.Fatal(err)
	}

	day, err := s.Day(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, a := range day {
		var texts []string
		for _, sess := range a.Sessions {
			for _, m := range sess.Messages {
				texts = append(texts, m.Content)
			}
		}
		got = append(got, a.Project+"="+strings.Join(texts, "|"))
	}
	want := []string{"dokaa=dokaa one|dokaa two|dokaa three", "jogai=jogai one|jogai two", "=a quick question"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Day =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if sess := day[0].Sessions[0]; sess.Tool != "claude-code" || sess.Project != "dokaa" || !sess.EndedAt.After(sess.StartedAt) {
		t.Errorf("session = %+v", sess)
	}
}
