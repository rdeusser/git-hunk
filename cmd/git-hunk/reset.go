package main

import (
	"context"
	"fmt"
)

// ResetCmd unstages all staged changes, or only the named files.
type ResetCmd struct {
	Paths []string `arg:"" name:"file" optional:"" help:"Files to unstage."`
}

func (c *ResetCmd) Help() string {
	return `This is equivalent to 'git reset HEAD'.

Examples:
  # Unstage all changes
  git-hunk reset

  # Unstage specific file
  git-hunk reset main.go`
}

func (c *ResetCmd) Run(ctx context.Context, g *Globals) error {
	if len(c.Paths) == 0 {
		if err := g.Git.Reset(ctx); err != nil {
			return err
		}

		fmt.Fprintln(g.Out, "Unstaged all changes.")

		return nil
	}

	if err := g.Git.ResetPaths(ctx, c.Paths...); err != nil {
		return err
	}

	fmt.Fprintf(g.Out, "Unstaged %d file(s).\n", len(c.Paths))

	return nil
}
