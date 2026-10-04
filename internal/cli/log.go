package cli

import (
	"fmt"
	"io"
	"os"
	"time"
)

const timestampLayout = "2006-01-02 15:04:05"

var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

func logf(format string, args ...any)    { writeLine(stdout, format, args...) }
func logErrf(format string, args ...any) { writeLine(stderr, format, args...) }

// Scheduled runs write to launchd log files, where a line without a timestamp
// cannot be tied to the run that failed.
func writeLine(w io.Writer, format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if !isTerminal(w) {
		line = time.Now().Format(timestampLayout) + " " + line
	}
	_, _ = fmt.Fprintln(w, line)
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
