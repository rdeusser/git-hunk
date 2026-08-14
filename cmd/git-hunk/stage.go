package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/rdeusser/git-hunk/diff"
	"github.com/rdeusser/git-hunk/patch"
)

// StageCmd stages specific lines from the working directory.
type StageCmd struct {
	Selections []string `arg:"" name:"FILE:LINES" help:"Lines to stage."`

	DryRun bool `help:"Show what would be staged without staging."`
}

func (c *StageCmd) Help() string {
	return `Lines are specified using FILE:LINES syntax where LINES can be:
  - A single line number: main.go:42
  - A range: main.go:10-20
  - Multiple ranges: main.go:10-20,30,40-50

Line numbers refer to the NEW file (after changes).
Use 'git-hunk diff' to see line numbers.

Examples:
  # Stage lines 10-20 from main.go
  git-hunk stage main.go:10-20

  # Stage multiple ranges from one file
  git-hunk stage main.go:10-20,30-40

  # Stage from multiple files
  git-hunk stage main.go:10-20 utils.go:5-15

  # Preview what would be staged
  git-hunk stage --dry-run main.go:10-20`
}

func (c *StageCmd) Run(ctx context.Context, g *Globals) error {
	selections, err := diff.ParseSelections(c.Selections)
	if err != nil {
		return fmt.Errorf("invalid selection: %w", err)
	}

	diffText, err := g.Git.Diff(ctx)
	if err != nil {
		return err
	}

	if diffText == "" {
		return fmt.Errorf("no unstaged changes")
	}

	parsed, err := diff.Parse(diffText)
	if err != nil {
		return err
	}

	patchBytes, err := patch.Generate(parsed, selections)
	if err != nil {
		return err
	}

	if c.DryRun {
		fmt.Fprint(g.Out, string(patchBytes))

		return nil
	}

	if err := g.Git.ApplyPatch(ctx, bytes.NewReader(patchBytes)); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	fmt.Fprintln(g.Out, "Changes staged successfully.")

	return nil
}
