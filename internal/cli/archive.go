package cli

import (
	"fmt"
	"os"

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

func (e *archiveEnv) refresh() (archive.RefreshResult, error) {
	return e.store.Refresh(e.sources, e.resolver, e.recapDir)
}

func archiveSessions(cfg *config.Config) {
	env, err := openArchive(cfg)
	if err != nil {
		logErrf("⚠ archive: %v", err)
		return
	}
	defer func() { _ = env.store.Close() }()
	res, err := env.refresh()
	if err != nil {
		logErrf("⚠ archive: %v", err)
	}
	if res.Ingested.Messages > 0 || res.Recaps > 0 {
		logf("Archived %d new message(s), indexed %d recap file(s)", res.Ingested.Messages, res.Recaps)
	}
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
