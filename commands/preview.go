package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/rdeusser/git-hunk/diff"
	"github.com/rdeusser/git-hunk/git"
	"github.com/rdeusser/git-hunk/output"
	"github.com/spf13/cobra"
)

// NewPreviewCmd creates the preview command.
func NewPreviewCmd() *cobra.Command {
	var showRaw bool

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Show staged changes",
		Long: `Show changes that are currently staged for commit.

This is equivalent to 'git diff --cached' but with hunk-style formatting.`,
		Example: `  # Show staged changes
  git-hunk preview

  # Show staged changes in JSON format
  git-hunk preview --json

  # Show raw unified diff
  git-hunk preview --raw`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPreview(cmd.Context(), cmd.OutOrStdout(), showRaw)
		},
	}

	cmd.Flags().BoolVar(
		&showRaw, "raw", false,
		"show raw unified diff",
	)

	return cmd
}

func runPreview(ctx context.Context, w io.Writer, showRaw bool) error {
	cfg := getConfig(ctx)
	executor := git.NewShellExecutor(cfg.WorkDir)

	diffText, err := executor.DiffCached(ctx)
	if err != nil {
		return err
	}

	if diffText == "" {
		if cfg.JSONOut {
			return output.FormatJSONEmpty(w)
		}

		fmt.Fprintln(w, "Nothing staged for commit.")

		return nil
	}

	parsed, err := diff.Parse(diffText)
	if err != nil {
		return err
	}

	if cfg.JSONOut {
		return output.FormatJSON(w, parsed)
	}

	if showRaw {
		return output.FormatRaw(w, parsed)
	}

	return output.FormatText(w, parsed, output.DefaultTextOptions())
}
