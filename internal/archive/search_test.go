package archive

import (
	"strings"
	"testing"
	"time"
)

func addDoc(t *testing.T, s *Store, id, kind, session, project, title string, ts time.Time, text string) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO docs(id, kind, session_id, project, git_branch, role, title, ts, text) VALUES(?, ?, ?, ?, 'main', 'user', ?, ?, ?)`,
		id, kind, session, project, title, ts.UnixMilli(), text); err != nil {
		t.Fatal(err)
	}
}

func searchFixture(t *testing.T) *Store {
	t.Helper()
	s, _ := openTemp(t)
	if _, err := s.db.Exec(`INSERT INTO projects VALUES ('/w/jogai', '/w/jogai', 'jogai'), ('/w/dokaa', '/w/dokaa', 'dokaa')`); err != nil {
		t.Fatal(err)
	}
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	addDoc(t, s, "m1", "message", "s1", "/w/jogai", "", day(1), "On a réglé les prompts TCC en neutralisant le cwd")
	addDoc(t, s, "m2", "message", "s1", "/w/jogai", "", day(1), "rejectConflictingOverwrite refuse d'écraser un recap")
	addDoc(t, s, "m3", "message", "s2", "/w/dokaa", "", day(20), "embed iframe du formulaire DOK-358")
	addDoc(t, s, "r1", "recap", "recap:2026-09-20", "", "2026-09-20 · Dokaa", day(20), "### Dokaa\nembed iframe validé avec Adrien")
	return s
}

func ids(hits []Hit) string {
	var out []string
	for _, h := range hits {
		out = append(out, h.ID)
	}
	return strings.Join(out, ",")
}

func TestSearch(t *testing.T) {
	s := searchFixture(t)
	tests := []struct {
		name string
		q    Query
		want string
	}{
		{"accents and prefixes", Query{Text: "regle neutral"}, "m1"},
		{"camelCase falls back to substrings", Query{Text: "overwrite"}, "m2"},
		{"no AND match falls back to OR", Query{Text: "tcc zzzzunknown"}, "m1"},
		{"recaps rank first", Query{Text: "iframe"}, "r1,m3"},
		{"kind filter", Query{Text: "iframe", Kind: "message"}, "m3"},
		{"project filter keeps matching recap sections", Query{Text: "iframe", Project: "dokaa"}, "r1,m3"},
		{"project filter excludes other projects", Query{Text: "iframe", Project: "jogai"}, ""},
		{"since", Query{Text: "iframe", Since: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)}, ""},
		{"current session excluded", Query{Text: "iframe", ExcludeSession: "s2"}, "r1"},
		{"query syntax is neutralized", Query{Text: `"DOK-358" c'est a:b (`}, "m3"},
		{"empty query", Query{Text: "  "}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hits, err := s.Search(tt.q)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if got := ids(hits); got != tt.want {
				t.Errorf("hits = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSearch_HitCarriesContext(t *testing.T) {
	s := searchFixture(t)
	hits, err := s.Search(Query{Text: "TCC"})
	if err != nil || len(hits) != 1 {
		t.Fatalf("Search = (%+v, %v)", hits, err)
	}
	h := hits[0]
	if h.Project != "jogai" || h.Branch != "main" || !strings.Contains(h.Snippet, "«TCC»") {
		t.Errorf("hit = %+v", h)
	}
}

func TestRead_StartsAtTheQuestionAndPaginates(t *testing.T) {
	s, _ := openTemp(t)
	at := func(m int) time.Time { return time.Date(2026, 9, 1, 10, m, 0, 0, time.UTC) }
	for i, d := range []struct{ id, role, text string }{
		{"q1", "user", "première question"},
		{"a1", "assistant", "première réponse"},
		{"q2", "user", "deuxième question"},
		{"a2", "assistant", strings.Repeat("longue réponse ", 50)},
	} {
		if _, err := s.db.Exec(`INSERT INTO docs(id, kind, session_id, project, git_branch, role, title, ts, text) VALUES(?, 'message', 's1', '', '', ?, '', ?, ?)`,
			d.id, d.role, at(i).UnixMilli(), d.text); err != nil {
			t.Fatal(err)
		}
	}

	page, err := s.Read("a1", 10_000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.Text, "première question") || !strings.Contains(page.Text, "deuxième question") || page.Next != "" {
		t.Errorf("Read(a1) = %+v, want the exchange from its question to the end", page)
	}

	page, err = s.Read("q1", 120)
	if err != nil || page.Next == "" {
		t.Fatalf("small budget = (%+v, %v), want a continuation", page, err)
	}
	rest, err := s.Read(page.Next, 10_000)
	if err != nil || !strings.Contains(rest.Text, "longue réponse") || strings.Contains(rest.Text, "première question") {
		t.Errorf("continuation = (%+v, %v), want the rest without rewinding", rest, err)
	}

	if _, err := s.Read("nope", 1000); err == nil {
		t.Error("an unknown id must be an error")
	}
}

func TestFormatHits(t *testing.T) {
	out := FormatHits([]Hit{
		{ID: "m1", Kind: "message", Project: "jogai", Branch: "main", Role: "user", Time: time.Date(2026, 9, 1, 10, 0, 0, 0, time.Local), Snippet: "a\nb «TCC»"},
		{ID: "recap:2026-09-20#1", Kind: "recap", Title: "2026-09-20 · Dokaa", Snippet: "embed"},
	})
	for _, want := range []string{"1. 2026-09-01 10:00 · jogai · main · user", "a b «TCC»", "id: m1", "2. recap 2026-09-20 · Dokaa", "id: recap:2026-09-20#1"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatHits missing %q:\n%s", want, out)
		}
	}
}
