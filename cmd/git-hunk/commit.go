package main

import (
	"context"
	"fmt"
)

// CommitCmd creates a commit with the currently staged changes.
type CommitCmd struct {
	Message string `short:"m" required:"" help:"Commit message."`
}

func (c *CommitCmd) Help() string {
	return `This is a thin wrapper around 'git commit' for convenience.

Examples:
  # Commit with a message
  git-hunk commit -m "add error handling"

  # Stage and commit in one command
  git-hunk stage main.go:10-20 && git-hunk commit -m "fix bug"`
}

func (c *CommitCmd) Run(ctx context.Context, g *Globals) error {
	diffText, err := g.Git.DiffCached(ctx)
	if err != nil {
		return err
	}

	if diffText == "" {
		return fmt.Errorf("nothing staged for commit")
	}

	if err := g.Git.Commit(ctx, c.Message); err != nil {
		return err
	}

	fmt.Fprintln(g.Out, "Committed successfully.")

	return nil
}
