package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// defaultBranches are the branches tried, in order, when no base is given.
var defaultBranches = []string{"main", "master"}

// git runs a git command and returns its trimmed stdout. Git's stderr is passed
// through so the user sees any diagnostics.
func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// repoRoot returns the absolute path of the current repository's root.
func repoRoot() (string, error) {
	return git("rev-parse", "--show-toplevel")
}

// branchExists reports whether a local branch of the given name exists.
func branchExists(branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

// defaultBase returns the branch to base new worktrees on when the user has not
// specified one.
func defaultBase() (string, error) {
	for _, branch := range defaultBranches {
		if branchExists(branch) {
			return branch, nil
		}
	}
	return "", fmt.Errorf(
		"could not find a default branch (tried %s), specify one with --base",
		strings.Join(defaultBranches, ", "),
	)
}

// worktreePath returns the path a worktree for branch should live at:
// ../$PROJECT__worktrees/<branch>, as a sibling of the repository root.
func worktreePath(root, branch string) string {
	project := filepath.Base(root)
	return filepath.Join(filepath.Dir(root), project+"__worktrees", sanitizeBranchName(branch))
}

// Git branch names cannot contain certain characters, so replace them with
// underscores. This is a simple approach; more complex sanitization may be
// needed for edge cases.
func sanitizeBranchName(branch string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(branch)
}

// AddWorktree creates a worktree for a new branch, based on base. If base is
// empty, the repository's default branch is used. It returns the path to the
// new worktree.
func AddWorktree(branch, base string) (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}

	if base == "" {
		if base, err = defaultBase(); err != nil {
			return "", err
		}
	} else if !branchExists(base) {
		return "", fmt.Errorf("base branch does not exist: %s", base)
	}

	path := worktreePath(root, branch)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("worktree already exists: %s", path)
	}

	// Git writes progress to stdout; send it to stderr so stdout only ever
	// carries the worktree path.
	cmd := exec.Command("git", "worktree", "add", "-b", branch, path, base)
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	return path, nil
}
