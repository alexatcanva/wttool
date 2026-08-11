/*
wttool is a tool for managing worktrees within a project. It more-or-less
supplies quick commands to generate a worktree under a ../$PROJECT__worktrees
directory. Very similar to how workmux does this.

Unlike workmux however, wttool does not depend on tmux/zellij, and thus allows
you to use your own tooling to edit/open the new worktree.
*/
package main

import (
	"os"

	"github.com/cavaliergopher/xflags"
)

func main() {
	os.Exit(xflags.Run(App))
}
