package main

import (
	"context"
	"fmt"

	"github.com/rdeusser/git-hunk/diff"
	"github.com/rdeusser/git-hunk/output"
)

// ListCmd lists the change groups that can be staged on their own.
type ListCmd struct {
	Paths  []string `arg:"" name:"file" optional:"" help:"Limit output to these files."`
	Staged bool     `help:"List groups in the index instead of the working tree."`
}

func (c *ListCmd) Help() string {
	return `Each row is one contiguous run of changed lines, which is the
smallest change that stages on its own. The first column is the selection
that stages that row, so a row can be pasted straight into 'git-hunk stage'.

A run mixing additions and deletions is one replacement and lists as a
single row, because staging half of it would describe a file that never
existed.

Examples:
  # List every stageable change
  git-hunk list

  # List changes in specific files
  git-hunk list main.go utils.go

  # List what is already staged
  git-hunk list --staged

  # JSON output for agents
  git-hunk list --json`
}

func (c *ListCmd) Run(ctx context.Context, g *Globals) error {
	var (
		diffText string
		err      error
	)

	if c.Staged {
		diffText, err = g.Git.DiffCached(ctx, c.Paths...)
	} else {
		diffText, err = g.Git.Diff(ctx, c.Paths...)
	}

	if err != nil {
		return err
	}

	if diffText == "" {
		if g.JSON {
			return output.FormatJSONGroups(g.Out, nil, nil)
		}

		return nil
	}

	parsed, err := diff.Parse(diffText)
	if err != nil {
		return err
	}

	if g.JSON {
		return output.FormatJSONGroups(g.Out, parsed, nil)
	}

	if err := output.FormatChangeGroups(g.Out, parsed); err != nil {
		return err
	}

	// A binary file has no lines to stage, so it contributes no rows. Say
	// so rather than leaving the caller to read the silence as "no change".
	for file := range parsed.Files() {
		if file.IsBinary {
			fmt.Fprintf(g.Out,
				"(%s is binary and cannot be staged by line)\n",
				file.Path(),
			)
		}
	}

	return nil
}
