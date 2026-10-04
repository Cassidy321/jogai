package parser

import (
	"fmt"
	"slices"
	"sort"
	"strings"
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
	failed := 0
	for _, p := range m.Parsers {
		sess, err := p.Sessions(since)
		if err != nil {
			failed++
			m.addWarning(fmt.Sprintf("%s: %v", p.Name(), err))
			continue
		}
		all = append(all, sess...)
	}
	if failed > 0 && failed == len(m.Parsers) {
		return nil, fmt.Errorf("every source failed: %s", strings.Join(m.Warnings(), "; "))
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].StartedAt.Before(all[j].StartedAt)
	})
	return all, nil
}

func (m *MultiParser) Warnings() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.warnings)
}

func (m *MultiParser) addWarning(w string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnings = append(m.warnings, w)
}
