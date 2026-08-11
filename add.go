package main

import (
	"fmt"
	"os"

	"github.com/cavaliergopher/xflags"
)

var (
	addBranch string
	addBase   string
)

// AddCommand creates a new worktree for a branch under
// ../$PROJECT__worktrees/<branch> and prints its path to stdout, so it can be
// composed with other tools:
//
//	code $(wttool add my-branch)
var AddCommand = xflags.NewCommand("add", "Create a worktree for a new branch").
	Synopsis(
		"add creates a worktree for BRANCH under ../$PROJECT__worktrees/BRANCH\n" +
			" and prints the path to the new worktree on stdout.",
	).
	Flags(
		xflags.String(
			&addBase,
			"base",
			"",
			"Branch to base the new worktree on (default: main or master)",
		).ShortName("b"),

		xflags.String(
			&addBranch,
			"BRANCH",
			"",
			"Name of the branch to create",
		).Positional().Required(),
	).
	HandleFunc(runAdd)

func runAdd(args []string) int {
	path, err := AddWorktree(addBranch, addBase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Only the path goes to stdout so the caller can capture it.
	fmt.Println(path)
	return 0
}
