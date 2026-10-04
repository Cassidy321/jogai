package parser

import "testing"

func TestProjectFromCwd(t *testing.T) {
	for cwd, want := range map[string]string{
		"":                     "unknown",
		"/":                    "unknown",
		"/Users/cassidy/jogai": "jogai",
	} {
		if got := projectFromCwd(cwd); got != want {
			t.Errorf("projectFromCwd(%q) = %q, want %q", cwd, got, want)
		}
	}
}
