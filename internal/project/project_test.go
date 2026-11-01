package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// t.TempDir lives under /var, a symlink on macOS: git answers with the real path.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolve(t *testing.T) {
	root := realTempDir(t)
	repo := filepath.Join(root, "jogai")
	sub := filepath.Join(repo, "internal", "cli")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-q")
	git(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	worktree := filepath.Join(root, "jogai-wt")
	git(t, repo, "worktree", "add", "-q", "-b", "wt", worktree)
	plain := filepath.Join(root, "notes")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	// Not /tmp: on Linux CI, t.TempDir itself lives there.
	r := &Resolver{Home: home, TempRoots: []string{"/agent-scratch"}}

	tests := []struct {
		cwd  string
		want Project
	}{
		{repo, Project{Key: repo, Name: "jogai"}},
		{sub, Project{Key: repo, Name: "jogai"}},
		{worktree, Project{Key: repo, Name: "jogai"}},
		{filepath.Join(repo, "deleted", "since"), Project{Key: repo, Name: "jogai"}},
		{plain, Project{Key: plain, Name: "notes"}},
		{filepath.Join(root, "gone", "lezom-front"), Project{Key: filepath.Join(root, "gone", "lezom-front"), Name: "lezom-front"}},
		{home, Project{}},
		{"/", Project{}},
		{"", Project{}},
		{"/agent-scratch/run-1", Project{}},
	}
	for _, tt := range tests {
		if got := r.Resolve(tt.cwd); got != tt.want {
			t.Errorf("Resolve(%q) = %+v, want %+v", tt.cwd, got, tt.want)
		}
	}
}

func TestNewResolver_DesktopScratchWorkspaceIsOutsideAnyProject(t *testing.T) {
	t.Setenv("HOME", "/Users/someone")
	r, err := NewResolver()
	if err != nil {
		t.Fatal(err)
	}
	cwd := "/Users/someone/Library/Application Support/Claude/scratch-workspaces/a/scratch-2026-10-28-ec2900"
	if got := r.Resolve(cwd); got != (Project{}) {
		t.Errorf("Resolve = %+v, want no project", got)
	}
}

func TestResolve_MissingFolderDoesNotJoinARepoInHome(t *testing.T) {
	home := realTempDir(t)
	git(t, home, "init", "-q")
	r := &Resolver{Home: home}
	gone := filepath.Join(home, "gone", "app")
	if got := r.Resolve(gone); got.Key != gone {
		t.Errorf("Resolve(%q) = %+v, want its own key", gone, got)
	}
}
