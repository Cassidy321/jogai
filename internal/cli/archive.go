package cli

import (
	"fmt"
	"os"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/parser"
	"github.com/Cassidy321/jogai/internal/project"
)

func archiveSessions(sources []parser.Source) {
	path, err := archive.DefaultPath()
	if err != nil {
		logErrf("⚠ archive: %v", err)
		return
	}
	store, err := archive.Open(path)
	if err != nil {
		logErrf("⚠ archive: %v", err)
		return
	}
	defer func() { _ = store.Close() }()
	resolver, err := project.NewResolver()
	if err != nil {
		logErrf("⚠ archive: %v", err)
		return
	}
	res, err := store.Ingest(sources, resolver)
	if err != nil {
		logErrf("⚠ archive: %v", err)
	}
	if res.Messages > 0 {
		logf("Archived %d new message(s)", res.Messages)
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
