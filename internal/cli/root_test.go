package cli

import (
	"testing"

	"github.com/alecthomas/kong"
)

func TestRunDayFlag(t *testing.T) {
	var app CLI
	parser, err := kong.New(&app)
	if err != nil {
		t.Fatalf("unexpected parser error: %v", err)
	}

	_, err = parser.Parse([]string{"run", "--day", "2026-04-10"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if app.Run.Day != "2026-04-10" {
		t.Fatalf("got day %q", app.Run.Day)
	}
}

func TestRunSinceAliasRemoved(t *testing.T) {
	var app CLI
	parser, err := kong.New(&app)
	if err != nil {
		t.Fatalf("unexpected parser error: %v", err)
	}

	_, err = parser.Parse([]string{"run", "--since", "2026-04-10"})
	if err == nil {
		t.Fatal("expected --since to be rejected in v0.5")
	}
}
