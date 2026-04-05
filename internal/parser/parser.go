package parser

import "time"

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	ID        string    `json:"id"`
	Tool      string    `json:"tool"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Project   string    `json:"project"`
	Messages  []Message `json:"messages"`
}

type Parser interface {
	Name() string
	Detect() bool
	Sessions(since time.Time) ([]Session, error)
}
