package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	OutputDir  string     `json:"output_dir"`
	DayEnd     *TimeOfDay `json:"day_end,omitempty"`
	Sources    []string   `json:"sources,omitempty"`
	Summarizer string     `json:"summarizer,omitempty"`
}

// TimeOfDay represents an hour-and-minute value in local time, used for the
// dev-day boundary.
type TimeOfDay struct {
	Hour   int
	Minute int
}

func ParseTimeOfDay(s string) (TimeOfDay, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return TimeOfDay{}, fmt.Errorf("invalid time %q: expected HH:MM", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return TimeOfDay{}, fmt.Errorf("invalid hour %q: must be 00-23", parts[0])
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return TimeOfDay{}, fmt.Errorf("invalid minute %q: must be 00-59", parts[1])
	}
	return TimeOfDay{Hour: h, Minute: m}, nil
}

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

func (t TimeOfDay) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *TimeOfDay) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("day_end must be a string: %w", err)
	}
	parsed, err := ParseTimeOfDay(s)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "jogai"), nil
}

func Load() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

var (
	ErrNotConfigured = fmt.Errorf("jogai not configured — run 'jogai init' first")
	ErrLocked        = errors.New("another jogai run is already in progress")
)
