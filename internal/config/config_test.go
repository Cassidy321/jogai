package config

import (
	"encoding/json"
	"testing"
)

func TestParseTimeOfDay(t *testing.T) {
	tests := []struct {
		in      string
		want    TimeOfDay
		wantErr bool
	}{
		{"00:00", TimeOfDay{0, 0}, false},
		{"05:00", TimeOfDay{5, 0}, false},
		{"05:30", TimeOfDay{5, 30}, false},
		{"19:13", TimeOfDay{19, 13}, false},
		{"23:59", TimeOfDay{23, 59}, false},
		{"24:00", TimeOfDay{}, true},
		{"05:60", TimeOfDay{}, true},
		{"-1:00", TimeOfDay{}, true},
		{"5:00", TimeOfDay{5, 0}, false},
		{"5", TimeOfDay{}, true},
		{"5:00:00", TimeOfDay{}, true},
		{"ab:cd", TimeOfDay{}, true},
		{"", TimeOfDay{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseTimeOfDay(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseTimeOfDay(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseTimeOfDay(%q) returned unexpected error: %v", tc.in, err)
				return
			}
			if got != tc.want {
				t.Errorf("ParseTimeOfDay(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestTimeOfDayString(t *testing.T) {
	tests := []struct {
		in   TimeOfDay
		want string
	}{
		{TimeOfDay{0, 0}, "00:00"},
		{TimeOfDay{5, 0}, "05:00"},
		{TimeOfDay{19, 13}, "19:13"},
		{TimeOfDay{23, 59}, "23:59"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.in.String(); got != tc.want {
				t.Errorf("%v.String() = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTimeOfDayJSONRoundtrip(t *testing.T) {
	original := TimeOfDay{Hour: 5, Minute: 30}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(data) != `"05:30"` {
		t.Errorf("Marshal = %s, want \"05:30\"", data)
	}

	var decoded TimeOfDay
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded != original {
		t.Errorf("roundtrip: got %v, want %v", decoded, original)
	}
}

func TestConfigMigrationMissingDayEnd(t *testing.T) {
	// v0.4-style config without day_end should load with zero-value TimeOfDay
	// (= 00:00 = calendar-day semantics).
	v04 := []byte(`{"output_dir": "/tmp/test"}`)

	var cfg Config
	if err := json.Unmarshal(v04, &cfg); err != nil {
		t.Fatalf("unmarshal v0.4 config: %v", err)
	}

	if cfg.OutputDir != "/tmp/test" {
		t.Errorf("OutputDir = %q, want /tmp/test", cfg.OutputDir)
	}
	if cfg.DayEnd != (TimeOfDay{}) {
		t.Errorf("DayEnd = %v, want zero value (00:00)", cfg.DayEnd)
	}
	if cfg.DayEnd.String() != "00:00" {
		t.Errorf("DayEnd.String() = %q, want 00:00", cfg.DayEnd.String())
	}
}

func TestConfigUnmarshalInvalidDayEnd(t *testing.T) {
	bad := []byte(`{"output_dir": "/tmp/test", "day_end": "25:00"}`)

	var cfg Config
	err := json.Unmarshal(bad, &cfg)
	if err == nil {
		t.Errorf("expected error for invalid day_end, got nil")
	}
}
