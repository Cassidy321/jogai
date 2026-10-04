package archive

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/lock"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
)

// Claude Code changes its transcript format without notice. Megabytes of new
// lines that yield no message mean jogai stopped understanding them, silently.
const formatDriftBytes = 5 << 20

type RefreshResult struct {
	Busy     bool
	Ingested Result
	Indexed  int
	Recaps   int
}

// Every Claude session starts its own `jogai mcp`: only one refreshes at a
// time. Searches pass wait 0 and use what is already archived; a recap run
// waits, since recapping from a half-imported archive writes partial recaps.
func (s *Store) Refresh(sources []parser.Source, resolver *project.Resolver, recapDir string, wait time.Duration) (RefreshResult, error) {
	release, err := lock.TryFor(filepath.Join(filepath.Dir(s.path), "refresh.lock"), wait)
	if errors.Is(err, lock.ErrBusy) {
		return RefreshResult{Busy: true}, nil
	}
	if err != nil {
		return RefreshResult{}, err
	}
	defer release()
	var res RefreshResult
	var errIngest, errIndex, errRecaps error
	res.Ingested, errIngest = s.Ingest(sources, resolver)
	res.Indexed, errIndex = s.indexMessages()
	res.Recaps, errRecaps = s.indexRecaps(recapDir)
	return res, errors.Join(errIngest, errIndex, errRecaps)
}

func (s *Store) updateFormatWarning(read map[string]int64, found map[string]int) error {
	var drifted []string
	anyFound := false
	for name, n := range read {
		if n >= formatDriftBytes && found[name] == 0 {
			drifted = append(drifted, name)
		}
		anyFound = anyFound || found[name] > 0
	}
	switch {
	case len(drifted) > 0:
		sort.Strings(drifted)
		return s.setMeta("format_warning", fmt.Sprintf("%s: megabytes of new transcripts on %s but no message found — the format may have changed; update jogai",
			strings.Join(drifted, ", "), time.Now().Format("2006-01-02")))
	case anyFound:
		_, err := s.db.Exec(`DELETE FROM meta WHERE key = 'format_warning'`)
		return err
	}
	return nil
}

func (s *Store) FormatWarning() (string, error) {
	var w string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'format_warning'`).Scan(&w)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return w, err
}
