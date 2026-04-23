package parser

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

type MultiParser struct {
	Parsers []Parser

	mu       sync.Mutex
	warnings []string
}

func (m *MultiParser) Name() string { return "multi" }

func (m *MultiParser) Detect() bool {
	for _, p := range m.Parsers {
		if p.Detect() {
			return true
		}
	}
	return false
}

func (m *MultiParser) Sessions(since time.Time) ([]Session, error) {
	m.mu.Lock()
	m.warnings = nil
	m.mu.Unlock()

	var all []Session
	for _, p := range m.Parsers {
		sess, err := p.Sessions(since)
		if err != nil {
			m.addWarning(fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		all = append(all, sess...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].StartedAt.Before(all[j].StartedAt)
	})
	return all, nil
}

func (m *MultiParser) Warnings() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.warnings))
	copy(out, m.warnings)
	return out
}

func (m *MultiParser) addWarning(w string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnings = append(m.warnings, w)
}
