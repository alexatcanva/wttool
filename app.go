package main

import "github.com/cavaliergopher/xflags"

// App is the root command for wttool. Subcommands are registered here.
var App = xflags.NewCommand("wttool", "Manage git worktrees for a project").
	Synopsis(
		"wttool creates and manages git worktrees under a sibling\n" +
			" ../$PROJECT__worktrees directory.",
	).
	Subcommands(
		AddCommand,
	)
