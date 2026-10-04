package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/output"
	"github.com/Cassidy321/jogai/internal/summary"
)

var errEditedByHand = errors.New("edited by hand since jogai wrote it")

// A recap edited in Obsidian is never replaced unless forced. Files written
// before day hashes existed have no recorded hash and can be replaced.
type guardedWriter struct {
	md     *output.Markdown
	days   map[string]lastrun.Day
	force  bool
	hashes map[string]string
}

func (w *guardedWriter) Write(s *summary.Summary) error {
	label := s.Date.Format(devday.LabelFormat)
	path := w.md.Path(s)
	if existing, err := os.ReadFile(path); err == nil && !w.force {
		if want := w.days[label].Hash; want != "" && want != hashOf(existing) {
			return fmt.Errorf("%s: %w", path, errEditedByHand)
		}
	}
	if err := w.md.Write(s); err != nil {
		return err
	}
	written, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	w.hashes[label] = hashOf(written)
	return nil
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
