package main

import (
	"context"
	"fmt"

	"github.com/rdeusser/git-hunk/diff"
	"github.com/rdeusser/git-hunk/output"
)

// DiffCmd shows unstaged (or staged) changes with line numbers.
type DiffCmd struct {
	Paths      []string `arg:"" name:"file" optional:"" help:"Limit output to these files."`
	Staged     bool     `help:"Show staged changes instead of unstaged."`
	Raw        bool     `help:"Show raw unified diff."`
	Files      bool     `help:"Show only file names."`
	Summary    bool     `help:"Show summary statistics."`
	StageHints bool     `help:"Show suggested git-hunk stage commands."`
}

func (c *DiffCmd) Help() string {
	return `Each line is prefixed with its line number in the new file,
making it easy to specify line ranges for staging.

Use --json for machine-readable output suitable for AI agents.

Examples:
  # Show all unstaged changes
  git-hunk diff

  # Show changes for specific files
  git-hunk diff main.go utils.go

  # Show staged changes
  git-hunk diff --staged

  # JSON output for AI agents
  git-hunk diff --json

  # Show suggested stage commands
  git-hunk diff --stage-hints`
}

func (c *DiffCmd) Run(ctx context.Context, g *Globals) error {
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

	// Get untracked files for awareness (only for unstaged diffs).
	var untracked []string
	if !c.Staged {
		status, statusErr := g.Git.Status(ctx)
		if statusErr == nil && len(status.UntrackedFiles) > 0 {
			untracked = status.UntrackedFiles
		}
	}

	if diffText == "" {
		if g.JSON {
			return output.FormatJSONEmptyWithUntracked(g.Out, untracked)
		}

		if len(untracked) > 0 {
			fmt.Fprintf(g.Out,
				"(%d untracked file(s) not shown - use git add)\n",
				len(untracked))
		}

		return nil
	}

	parsed, err := diff.Parse(diffText)
	if err != nil {
		return err
	}

	if g.JSON {
		return output.FormatJSONWithUntracked(g.Out, parsed, untracked)
	}

	// Handle different output modes.
	var formatErr error

	switch {
	case c.Raw:
		formatErr = output.FormatRaw(g.Out, parsed)
	case c.Files:
		formatErr = output.FormatFileList(g.Out, parsed)
	case c.Summary:
		formatErr = output.FormatTextSummary(g.Out, parsed)
	case c.StageHints:
		formatErr = output.FormatStagingCommands(g.Out, parsed)
	default:
		formatErr = output.FormatText(g.Out, parsed, output.DefaultTextOptions())
	}

	if formatErr != nil {
		return formatErr
	}

	// Show note about untracked files.
	if len(untracked) > 0 && !c.Raw {
		fmt.Fprintf(g.Out,
			"\n(%d untracked file(s) not shown - use git add)\n",
			len(untracked))
	}

	return nil
}
