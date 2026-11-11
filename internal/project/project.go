package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Key is "" for work outside any project: sessions started from the home
// folder or a temp dir (one-off questions, agent scratchpads).
type Project struct {
	Key  string
	Name string
}

type Resolver struct {
	Home      string
	TempRoots []string
}

func NewResolver() (*Resolver, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return &Resolver{
		Home: home,
		TempRoots: []string{
			"/tmp", "/private/tmp", "/var/folders", "/private/var/folders", filepath.Clean(os.TempDir()),
			// Where the Claude desktop app runs a session started without a project folder.
			filepath.Join(home, "Library", "Application Support", "Claude", "scratch-workspaces"),
		},
	}, nil
}

func (r *Resolver) Resolve(cwd string) Project {
	if r.outside(cwd) {
		return Project{}
	}
	key := filepath.Clean(cwd)
	if root, ok := r.enclosingRepo(key); ok {
		key = root
	}
	return Project{Key: key, Name: filepath.Base(key)}
}

// A transcript can be read long after it was written: by then the session's
// folder may be deleted or on another branch, and only a parent tells the repo.
func (r *Resolver) enclosingRepo(dir string) (string, bool) {
	for ; !r.outside(dir); dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			root, err := repoRoot(dir)
			return root, err == nil
		}
	}
	return "", false
}

func (r *Resolver) outside(cwd string) bool {
	clean := filepath.Clean(cwd)
	if cwd == "" || clean == "/" || clean == filepath.Clean(r.Home) {
		return true
	}
	for _, tmp := range r.TempRoots {
		if clean == tmp || strings.HasPrefix(clean, tmp+"/") {
			return true
		}
	}
	return false
}

// The common dir of a worktree is the main repository's .git, so all worktrees
// of a repo share one project. Submodules have no .git there: use their top level.
func repoRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", err
	}
	common := strings.TrimSpace(string(out))
	if filepath.Base(common) == ".git" {
		return filepath.Dir(common), nil
	}
	out, err = exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
