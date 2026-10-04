package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/archive"
	"github.com/Cassidy321/jogai/internal/config"
	"github.com/Cassidy321/jogai/internal/devday"
)

type SearchCmd struct {
	Query   []string `arg:"" help:"Words to look for."`
	Project string   `help:"Only this project."`
	Recaps  bool     `help:"Only daily recaps."`
	Since   string   `help:"From this day (YYYY-MM-DD)."`
	Limit   int      `default:"10" help:"Maximum number of results."`
}

func (c *SearchCmd) Run() error {
	cfg, _ := config.Load()
	env, err := openArchive(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = env.store.Close() }()
	if _, err := env.refresh(); err != nil {
		logErrf("⚠ archive: %v", err)
	}
	q := archive.Query{Text: strings.Join(c.Query, " "), Project: c.Project, Limit: c.Limit}
	if c.Recaps {
		q.Kind = "recap"
	}
	if c.Since != "" {
		if q.Since, err = time.ParseInLocation(devday.LabelFormat, c.Since, time.Local); err != nil {
			return fmt.Errorf("invalid --since %q — expected YYYY-MM-DD", c.Since)
		}
	}
	hits, err := env.store.Search(q)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Println("No results.")
		return nil
	}
	fmt.Print(archive.FormatHits(hits))
	return nil
}
