package lastrun

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Cassidy321/jogai/internal/config"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusPartial Status = "partial"
	StatusError   Status = "error"
	StatusEmpty   Status = "empty"
	StatusRefused Status = "refused"
)

type Record struct {
	RanAt    time.Time `json:"ran_at"`
	DevDay   string    `json:"dev_day"`
	Status   Status    `json:"status"`
	Error    string    `json:"error,omitempty"`
	Warnings []string  `json:"warnings,omitempty"`
}

func path() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last_run.json"), nil
}

func Save(r *Record) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal last_run: %w", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return fmt.Errorf("write last_run: %w", err)
	}
	return nil
}

func Load() (*Record, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read last_run: %w", err)
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse last_run: %w", err)
	}
	return &r, nil
}

type Day struct {
	Status    Status    `json:"status"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	Hash      string    `json:"hash,omitempty"`
}

// Refused days are not retried: the model would refuse the same content again.
func (d Day) NeedsRetry() bool { return d.Status == StatusError }

func daysPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "days.json"), nil
}

func LoadDays() (map[string]Day, error) {
	p, err := daysPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return map[string]Day{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read days: %w", err)
	}
	days := map[string]Day{}
	if err := json.Unmarshal(data, &days); err != nil {
		return nil, fmt.Errorf("parse days: %w", err)
	}
	return days, nil
}

func SaveDays(days map[string]Day) error {
	p, err := daysPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(days, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal days: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".days-*.json")
	if err != nil {
		return fmt.Errorf("create temp days: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write days: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("close days: %w", err)
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("save days: %w", err)
	}
	return nil
}
