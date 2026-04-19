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

func TestConfigUnconfiguredDayEnd(t *testing.T) {
	// A config without day_end (legacy or never-configured) unmarshals to a nil
	// DayEnd so callers can distinguish "never set" from "explicitly 00:00".
	raw := []byte(`{"output_dir": "/tmp/test"}`)

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.OutputDir != "/tmp/test" {
		t.Errorf("OutputDir = %q, want /tmp/test", cfg.OutputDir)
	}
	if cfg.DayEnd != nil {
		t.Errorf("DayEnd = %v, want nil", cfg.DayEnd)
	}
}

func TestConfigSetDayEndRoundtrip(t *testing.T) {
	t0 := TimeOfDay{Hour: 5, Minute: 0}
	cfg := Config{OutputDir: "/tmp/test", DayEnd: &t0}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.DayEnd == nil || *decoded.DayEnd != t0 {
		t.Errorf("roundtrip: got %v, want &%v", decoded.DayEnd, t0)
	}
}

func TestConfigMarshalOmitsNilDayEnd(t *testing.T) {
	cfg := Config{OutputDir: "/tmp/test"}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != `{"output_dir":"/tmp/test"}` {
		t.Errorf("expected day_end to be omitted, got: %s", data)
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
