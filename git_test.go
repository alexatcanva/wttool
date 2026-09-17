package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGit runs a git command in dir and fails the test if it errors.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// initRepo creates a git repo in dir with an initial commit on main.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-q", "-m", "init")
}

// chdir changes the working directory to dir and returns a func to restore it.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSanitizeBranchName(t *testing.T) {
	cases := map[string]string{
		"feature/foo":  "feature_foo",
		"a b":          "a_b",
		"weird*?name":  "weird__name",
		"plain":        "plain",
		`back\slash`:   "back_slash",
		`quote"pipe|x`: "quote_pipe_x",
	}
	for in, want := range cases {
		if got := sanitizeBranchName(in); got != want {
			t.Errorf("sanitizeBranchName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	root := "/home/user/myproject"
	got := worktreePath(root, "feature/foo")
	want := "/home/user/myproject__worktrees/feature_foo"
	if got != want {
		t.Errorf("worktreePath(%q, %q) = %q, want %q", root, "feature/foo", got, want)
	}
}

// TestRepoRootFromWithinWorktree verifies that repoRoot resolves to the real
// repository's root even when the current directory is inside a worktree
// created from it, rather than treating the worktree itself as the root.
func TestRepoRootFromWithinWorktree(t *testing.T) {
	tmp := t.TempDir()
	realRoot := filepath.Join(tmp, "realrepo")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, realRoot)

	wtPath := filepath.Join(tmp, "realrepo__worktrees", "some-branch")
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, realRoot, "worktree", "add", "-q", "-b", "some-branch", wtPath, "main")

	resolvedRoot, err := filepath.EvalSymlinks(realRoot)
	if err != nil {
		t.Fatal(err)
	}

	chdir(t, wtPath)
	got, err := repoRoot()
	if err != nil {
		t.Fatalf("repoRoot() error: %v", err)
	}
	resolvedGot, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedGot != resolvedRoot {
		t.Errorf("repoRoot() from within worktree = %q, want %q", resolvedGot, resolvedRoot)
	}
}

// TestAddWorktreeFromWithinWorktree is a regression test for a bug where
// running AddWorktree from inside a worktree it had previously created would
// nest the new worktree inside the existing one, instead of creating it as a
// sibling of the real repository.
func TestAddWorktreeFromWithinWorktree(t *testing.T) {
	tmp := t.TempDir()
	realRoot := filepath.Join(tmp, "realrepo")
	if err := os.Mkdir(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, realRoot)

	chdir(t, realRoot)
	firstPath, err := AddWorktree("first-branch", "")
	if err != nil {
		t.Fatalf("AddWorktree(first-branch) error: %v", err)
	}
	wantFirst := filepath.Join(tmp, "realrepo__worktrees", "first-branch")
	if firstPath != wantFirst {
		t.Fatalf("first worktree path = %q, want %q", firstPath, wantFirst)
	}

	// Now run AddWorktree again, but from within the worktree just created.
	chdir(t, firstPath)
	secondPath, err := AddWorktree("second-branch", "first-branch")
	if err != nil {
		t.Fatalf("AddWorktree(second-branch) from within worktree error: %v", err)
	}

	wantSecond := filepath.Join(tmp, "realrepo__worktrees", "second-branch")
	if secondPath != wantSecond {
		t.Errorf("second worktree path = %q, want %q (should be a sibling of the real repo, not nested under the first worktree)", secondPath, wantSecond)
	}
	if _, err := os.Stat(filepath.Join(firstPath, "realrepo__worktrees")); err == nil {
		t.Errorf("second worktree was nested inside the first worktree at %q", firstPath)
	}
}

// TestResolveBaseRefPrefersFreshOrigin verifies that resolveBaseRef fetches
// and prefers origin/<base> over a stale local branch, so new worktrees are
// based on up-to-date history without requiring the user to manually pull.
func TestResolveBaseRefPrefersFreshOrigin(t *testing.T) {
	tmp := t.TempDir()

	// Bare "remote" repo.
	remoteDir := filepath.Join(tmp, "remote.git")
	if err := os.Mkdir(remoteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, remoteDir, "init", "-q", "--bare", "-b", "main")

	// Local clone (our "repo" under test).
	localDir := filepath.Join(tmp, "local")
	runGit(t, tmp, "clone", "-q", remoteDir, localDir)
	runGit(t, localDir, "config", "user.email", "test@example.com")
	runGit(t, localDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, localDir, "add", "README.md")
	runGit(t, localDir, "commit", "-q", "-m", "init")
	runGit(t, localDir, "push", "-q", "origin", "main")

	// A second clone pushes a new commit to the remote, simulating a
	// teammate's change that our stale local clone hasn't seen yet.
	otherDir := filepath.Join(tmp, "other")
	runGit(t, tmp, "clone", "-q", remoteDir, otherDir)
	runGit(t, otherDir, "config", "user.email", "test@example.com")
	runGit(t, otherDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(otherDir, "NEW.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, otherDir, "add", "NEW.md")
	runGit(t, otherDir, "commit", "-q", "-m", "fresh commit")
	runGit(t, otherDir, "push", "-q", "origin", "main")

	localMain := strings.TrimSpace(runGit(t, localDir, "rev-parse", "main"))
	remoteMain := strings.TrimSpace(runGit(t, remoteDir, "rev-parse", "main"))
	if localMain == remoteMain {
		t.Fatal("test setup error: local main should be stale relative to origin")
	}

	chdir(t, localDir)
	ref := resolveBaseRef("main")
	if ref != "origin/main" {
		t.Fatalf("resolveBaseRef(main) = %q, want %q", ref, "origin/main")
	}

	resolvedRef, err := git("rev-parse", ref)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedRef != remoteMain {
		t.Errorf("resolveBaseRef result resolves to %q, want up-to-date origin main %q", resolvedRef, remoteMain)
	}
}

// TestResolveBaseRefNoRemote verifies that resolveBaseRef falls back to the
// local branch name when there is no origin remote to fetch from.
func TestResolveBaseRefNoRemote(t *testing.T) {
	tmp := t.TempDir()
	initRepo(t, tmp)
	chdir(t, tmp)

	if got := resolveBaseRef("main"); got != "main" {
		t.Errorf("resolveBaseRef(main) with no remote = %q, want %q", got, "main")
	}
}
