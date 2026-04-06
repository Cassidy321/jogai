package scheduler

import (
	"strings"
	"testing"
)

func TestGeneratePlistDaily(t *testing.T) {
	sched := Schedule{Hour: 9, Minute: 0, Weekday: -1, MonthDay: -1}
	plist, err := generatePlist("daily", sched, "/usr/local/bin/jogai", "/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	s := string(plist)

	mustContain := []string{
		"<string>com.jogai.daily</string>",
		"<string>/usr/local/bin/jogai</string>",
		"<string>run</string>",
		"<string>--period</string>",
		"<string>daily</string>",
		"<key>Hour</key>",
		"<integer>9</integer>",
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

	mustNotContain := []string{"<key>Weekday</key>", "<key>Day</key>"}
	for _, bad := range mustNotContain {
		if strings.Contains(s, bad) {
			t.Errorf("plist should not contain %q for daily", bad)
		}
	}
}

func TestGeneratePlistWeekly(t *testing.T) {
	sched := Schedule{Hour: 18, Minute: 30, Weekday: 5, MonthDay: -1}
	plist, err := generatePlist("weekly", sched, "/usr/local/bin/jogai", "/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	s := string(plist)

	if !strings.Contains(s, "<string>com.jogai.weekly</string>") {
		t.Error("missing label")
	}
	if !strings.Contains(s, "<key>Weekday</key>") {
		t.Error("missing weekday key")
	}
	if !strings.Contains(s, "<integer>5</integer>") {
		t.Error("missing weekday value")
	}
	if strings.Contains(s, "<key>Day</key>") {
		t.Error("should not have Day for weekly")
	}
}

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

func TestGeneratePlistMonthly(t *testing.T) {
	sched := Schedule{Hour: 9, Minute: 0, Weekday: -1, MonthDay: 15}
	plist, err := generatePlist("monthly", sched, "/usr/local/bin/jogai", "/tmp/logs")
	if err != nil {
		t.Fatal(err)
	}
	s := string(plist)

	if !strings.Contains(s, "<key>Day</key>") {
		t.Error("missing Day key")
	}
	if !strings.Contains(s, "<integer>15</integer>") {
		t.Error("missing day value")
	}
	if strings.Contains(s, "<key>Weekday</key>") {
		t.Error("should not have Weekday for monthly")
	}
}
