package main

import (
	"context"
	"fmt"

	"github.com/rdeusser/git-hunk/diff"
	"github.com/rdeusser/git-hunk/output"
)

// PreviewCmd shows changes that are currently staged for commit.
type PreviewCmd struct {
	Raw bool `help:"Show raw unified diff."`
}

func (c *PreviewCmd) Help() string {
	return `This is equivalent to 'git diff --cached' but with hunk-style formatting.

Examples:
  # Show staged changes
  git-hunk preview

  # Show staged changes in JSON format
  git-hunk preview --json

  # Show raw unified diff
  git-hunk preview --raw`
}

func (c *PreviewCmd) Run(ctx context.Context, g *Globals) error {
	diffText, err := g.Git.DiffCached(ctx)
	if err != nil {
		return err
	}

	if diffText == "" {
		if g.JSON {
			return output.FormatJSONEmpty(g.Out)
		}

		fmt.Fprintln(g.Out, "Nothing staged for commit.")

		return nil
	}

	parsed, err := diff.Parse(diffText)
	if err != nil {
		return err
	}

	if g.JSON {
		return output.FormatJSON(g.Out, parsed)
	}

	if c.Raw {
		return output.FormatRaw(g.Out, parsed)
	}

	return output.FormatText(g.Out, parsed, output.DefaultTextOptions())
}
