package archive

import (
	"fmt"
	"sort"
	"time"

	"github.com/Cassidy321/jogai/internal/parser"
)

type Activity struct {
	Project  string
	Sessions []parser.Session
	messages int
}

// Cleaned at read time with the same rules as the index (parser.Clean): the
// archive keeps raw records so the rules can change later.
func (s *Store) Day(since, until time.Time) ([]Activity, error) {
	rows, err := s.db.Query(`SELECT m.session_id, coalesce(se.source, ''), m.ts, m.role, m.text, m.answers, m.is_meta, m.is_compact, m.origin,
			m.project, coalesce((SELECT name FROM projects p WHERE p.key = m.project LIMIT 1), '')
		FROM messages m LEFT JOIN sessions se ON se.id = m.session_id
		WHERE m.ts >= ? AND m.ts < ? ORDER BY m.ts, m.rowid`, since.UnixMilli(), until.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("read day: %w", err)
	}
	defer func() { _ = rows.Close() }()
	byKey := map[string]*Activity{}
	sessionAt := map[string]int{}
	var keys []string
	for rows.Next() {
		var r parser.Record
		var source, key, name string
		var ts int64
		var meta, compact int
		if err := rows.Scan(&r.SessionID, &source, &ts, &r.Role, &r.Text, &r.Answers, &meta, &compact, &r.Origin, &key, &name); err != nil {
			return nil, fmt.Errorf("read day: %w", err)
		}
		r.IsMeta, r.IsCompact = meta == 1, compact == 1
		text := parser.Clean(r)
		if text == "" {
			continue
		}
		a, ok := byKey[key]
		if !ok {
			a = &Activity{Project: name}
			byKey[key] = a
			keys = append(keys, key)
		}
		at := time.UnixMilli(ts)
		i, ok := sessionAt[key+"\x00"+r.SessionID]
		if !ok {
			i = len(a.Sessions)
			sessionAt[key+"\x00"+r.SessionID] = i
			a.Sessions = append(a.Sessions, parser.Session{ID: r.SessionID, Tool: source, Project: name, StartedAt: at})
		}
		a.Sessions[i].Messages = append(a.Sessions[i].Messages, parser.Message{Role: r.Role, Content: text, Timestamp: at})
		a.Sessions[i].EndedAt = at
		a.messages++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Activity, 0, len(keys))
	for _, k := range keys {
		out = append(out, *byKey[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Project == "") != (out[j].Project == "") {
			return out[j].Project == ""
		}
		return out[i].messages > out[j].messages
	})
	return out, nil
}
