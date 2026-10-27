package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
)

type archiveEnv struct {
	store    *archive.Store
	sources  []parser.Source
	resolver *project.Resolver
	recapDir string
}

func openArchive(cfg *config.Config) (*archiveEnv, error) {
	if cfg == nil {
		cfg = &config.Config{}
	}
	sources, err := activeSources(cfg)
	if err != nil {
		return nil, err
	}
	resolver, err := project.NewResolver()
	if err != nil {
		return nil, err
	}
	path, err := archive.DefaultPath()
	if err != nil {
		return nil, err
	}
	store, err := archive.Open(path)
	if err != nil {
		return nil, err
	}
	return &archiveEnv{store: store, sources: sources, resolver: resolver, recapDir: cfg.OutputDir}, nil
}

func (e *archiveEnv) refresh(wait time.Duration) (archive.RefreshResult, error) {
	return e.store.Refresh(e.sources, e.resolver, e.recapDir, wait)
}

// Recaps are left to the searches, which index them before searching. macOS
// does not let the scheduled run list a recap folder under ~/Documents, and
// trying would mark every recap partial.
func refreshArchive(env *archiveEnv) []string {
	res, err := env.store.Refresh(env.sources, env.resolver, "", 5*time.Minute)
	if res.Ingested.Messages > 0 {
		logf("Archived %d new message(s)", res.Ingested.Messages)
	}
	if err == nil {
		return nil
	}
	logErrf("⚠ archive: %v", err)
	first, _, _ := strings.Cut(err.Error(), "\n")
	return []string{"some sessions could not be read: " + first}
}

func printArchiveLine() bool {
	path, err := archive.DefaultPath()
	if err != nil {
		fmt.Printf("  Archive:    ✗ %v\n", err)
		return false
	}
	if _, err := os.Stat(path); err != nil {
		fmt.Println("  Archive:    empty — the next run fills it")
		return true
	}
	store, err := archive.Open(path)
	if err != nil {
		fmt.Printf("  Archive:    ✗ %v\n", err)
		return false
	}
	defer func() { _ = store.Close() }()
	st, err := store.Stats()
	if err != nil {
		fmt.Printf("  Archive:    ✗ %v\n", err)
		return false
	}
	fmt.Printf("  Archive:    %d messages from %d sessions, updated %s\n",
		st.Messages, st.Sessions, st.LastIngest.Format("2006-01-02 15:04"))
	return true
}
