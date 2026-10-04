package cli

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/summary"
)

func recapFor(content string) *summary.Summary {
	day := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	return &summary.Summary{Date: day, WindowStart: day, WindowEnd: day.AddDate(0, 0, 1), Content: content}
}

func TestGuardedWriter(t *testing.T) {
	md := output.NewMarkdown(t.TempDir())
	days := map[string]lastrun.Day{}
	w := &guardedWriter{md: md, days: days, hashes: map[string]string{}}

	if err := w.Write(recapFor("first")); err != nil {
		t.Fatal(err)
	}
	days["2026-04-06"] = lastrun.Day{Hash: w.hashes["2026-04-06"]}
	if err := w.Write(recapFor("second")); err != nil {
		t.Fatalf("rewriting jogai's own file: %v", err)
	}
	days["2026-04-06"] = lastrun.Day{Hash: w.hashes["2026-04-06"]}

	path := md.Path(recapFor(""))
	edited, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited = append(edited, []byte("\nmy own note\n")...)
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(recapFor("third")); !errors.Is(err, errEditedByHand) {
		t.Fatalf("Write over an edited recap = %v, want errEditedByHand", err)
	}
	if kept, _ := os.ReadFile(path); string(kept) != string(edited) {
		t.Error("the edited recap must stay untouched")
	}

	w.force = true
	if err := w.Write(recapFor("forced")); err != nil {
		t.Fatalf("forced write: %v", err)
	}

	w.force = false
	delete(days, "2026-04-06")
	if err := w.Write(recapFor("from an older version")); err != nil {
		t.Errorf("a file without a recorded hash can be replaced: %v", err)
	}
}
