package update

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBrewFor(t *testing.T) {
	tests := []struct {
		path string
		brew string
		ok   bool
	}{
		{"/opt/homebrew/Cellar/jogai/0.6.0/bin/jogai", "/opt/homebrew/bin/brew", true},
		{"/usr/local/Cellar/jogai/0.6.0/bin/jogai", "/usr/local/bin/brew", true},
		{"/Users/me/go/bin/jogai", "", false},
	}
	for _, tt := range tests {
		brew, ok := brewFor(tt.path)
		if brew != tt.brew || ok != tt.ok {
			t.Errorf("brewFor(%q) = (%q, %v), want (%q, %v)", tt.path, brew, ok, tt.brew, tt.ok)
		}
	}
}

func TestDue(t *testing.T) {
	stamp := filepath.Join(t.TempDir(), "last_update_check")
	now := time.Now()
	if !due(stamp, now) {
		t.Error("without a stamp an update check is due")
	}
	if err := touch(stamp, now); err != nil {
		t.Fatal(err)
	}
	if due(stamp, now.Add(time.Hour)) {
		t.Error("checked an hour ago: not due")
	}
	if !due(stamp, now.Add(25*time.Hour)) {
		t.Error("checked yesterday: due")
	}
}
