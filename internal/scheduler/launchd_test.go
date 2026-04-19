package scheduler

import (
	"strings"
	"testing"

	"github.com/Cassidy321/jogai/internal/config"
)

func TestIsTempBinary(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/bin/jogai", false},
		{"/usr/local/bin/jogai", false},
		{"/Users/cassidy/jogai/bin/jogai", false},
		{"/var/folders/xx/yy/T/go-build123/jogai", true},
		{"/tmp/go-build456/exe/jogai", true},
		{"/private/var/folders/zz/T/go-build789/jogai", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isTempBinary(tt.path); got != tt.want {
				t.Errorf("isTempBinary(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestGeneratePlist(t *testing.T) {
	dayEnd := config.TimeOfDay{Hour: 5, Minute: 0}
	plist, err := generatePlist(dayEnd, "/usr/local/bin/jogai", "/Users/test/.local/bin", "/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	s := string(plist)

	mustContain := []string{
		"<string>com.jogai.daily</string>",
		"<string>/usr/bin/caffeinate</string>",
		"<string>/usr/local/bin/jogai</string>",
		"<string>/Users/test/.local/bin:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>",
		"<string>run</string>",
		"<key>Hour</key>",
		"<integer>5</integer>",
		"<key>Minute</key>",
		"<integer>0</integer>",
		"<string>/tmp/logs/daily.out.log</string>",
		"<string>/tmp/logs/daily.err.log</string>",
	}
	for _, want := range mustContain {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q", want)
		}
	}

	mustNotContain := []string{"--scheduled", "--at", "--period"}
	for _, bad := range mustNotContain {
		if strings.Contains(s, bad) {
			t.Errorf("plist should not contain %q", bad)
		}
	}
}
