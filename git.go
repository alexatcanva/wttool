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

// repoRoot returns the absolute path of the main repository's root, even when
// run from inside a worktree created by this tool. This ensures new worktrees
// are always created as siblings of the real repository rather than nested
// inside another worktree.
func repoRoot() (string, error) {
	commonDir, err := git("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Dir(commonDir), nil
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

// resolveBaseRef returns the ref a new worktree should start from for the
// given base branch. When an "origin" remote exists, it fetches the latest
// state of base from origin and prefers origin/<base>, so worktrees start
// from up-to-date history instead of a possibly stale local branch. It falls
// back to the local branch name if there is no origin remote, the fetch
// fails (e.g. no network), or origin has no such branch.
func resolveBaseRef(base string) string {
	if _, err := git("remote", "get-url", "origin"); err != nil {
		return base
	}
	if _, err := git("fetch", "origin", base); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to fetch origin/%s, using local branch: %v\n", base, err)
		return base
	}
	remoteBranch := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+base)
	if remoteBranch.Run() != nil {
		return base
	}
	return "origin/" + base
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

	baseRef := resolveBaseRef(base)

	path := worktreePath(root, branch)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("worktree already exists: %s", path)
	}

	// Git writes progress to stdout; send it to stderr so stdout only ever
	// carries the worktree path.
	cmd := exec.Command("git", "worktree", "add", "-b", branch, path, baseRef)
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	return path, nil
}
