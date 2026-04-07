package output

import "github.com/Cassidy321/jogai/internal/summary"

type Adapter interface {
	Write(s *summary.Summary) error
}
