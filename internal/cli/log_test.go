package cli

import (
	"bytes"
	"regexp"
	"testing"
)

func TestWriteLine_TimestampsNonTerminalOutput(t *testing.T) {
	var buf bytes.Buffer
	writeLine(&buf, "dev day %s: recap written", "2026-04-17")
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} dev day 2026-04-17: recap written\n$`)
	if !re.MatchString(buf.String()) {
		t.Errorf("line = %q, want a timestamped line", buf.String())
	}
}
