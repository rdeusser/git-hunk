package main

import (
	"context"
	"fmt"
	"os"
)

// ApplyPatchCmd applies a unified diff patch to the staging area.
type ApplyPatchCmd struct {
	File string `arg:"" optional:"" help:"Patch file to apply; reads stdin when omitted."`
}

func (c *ApplyPatchCmd) Help() string {
	return `If no file is specified, reads from stdin.
This is equivalent to 'git apply --cached'.

Examples:
  # Apply patch from file
  git-hunk apply-patch changes.patch

  # Apply patch from stdin (useful for piping)
  cat changes.patch | git-hunk apply-patch`
}

func (c *ApplyPatchCmd) Run(ctx context.Context, g *Globals) error {
	r := g.In

	if c.File != "" {
		f, err := os.Open(c.File)
		if err != nil {
			return fmt.Errorf("failed to open patch file: %w", err)
		}
		defer f.Close()

		r = f
	}

	if err := g.Git.ApplyPatch(ctx, r); err != nil {
		return err
	}

	fmt.Fprintln(g.Out, "Patch applied to staging area.")

	return nil
}
