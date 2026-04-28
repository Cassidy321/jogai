package cli

import (
	"testing"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestContainsString(t *testing.T) {
	if !containsString([]string{"a", "b"}, "b") {
		t.Error("expected true")
	}
	if containsString(nil, "x") {
		t.Error("expected false")
	}
}

func TestPrintSourcesStatus_NoPanic(t *testing.T) {
	// Smoke: never panics for the various config combinations we care about.
	cases := []*config.Config{
		nil,
		{},
		{Sources: []string{"claude-code"}},
		{Sources: []string{"claude-code", "codex"}},
		{Sources: []string{"codex"}},
	}
	for _, cfg := range cases {
		printSourcesStatus(detectedSources{claudeCode: true, codex: true}, cfg)
		printSourcesStatus(detectedSources{claudeCode: false, codex: true}, cfg)
	}
}
