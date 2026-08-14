// Command git-hunk stages individual lines of a git diff.
package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/rdeusser/git-hunk/git"
)

const description = `git-hunk enables precise, line-level staging for git commits.

Designed for AI agents that need to make surgical changes to codebases,
git-hunk provides a simple interface for selecting and staging specific lines
from a diff.

Examples:
  # Show all changes with line numbers
  git-hunk diff

  # Show changes in JSON format (for agents)
  git-hunk diff --json

  # Stage specific lines from a file
  git-hunk stage main.go:10-20

  # Stage multiple ranges from multiple files
  git-hunk stage main.go:10-20,30-40 utils.go:5-15

  # Preview what's staged
  git-hunk preview

  # Commit staged changes
  git-hunk commit -m "add error handling"

  # Apply a patch directly to staging
  git-hunk apply-patch < changes.diff`

// CLI is the command grammar. Kong derives every command name, flag, and
// help string from the tags below.
type CLI struct {
	Dir        string        `short:"C" placeholder:"PATH" help:"Run as if git was started in this directory."`
	JSON       bool          `help:"Output in JSON format (for machine consumption)."`
	Diff       DiffCmd       `cmd:"" help:"Show changes with line numbers."`
	Stage      StageCmd      `cmd:"" help:"Stage specific lines."`
	Preview    PreviewCmd    `cmd:"" help:"Show staged changes."`
	Commit     CommitCmd     `cmd:"" help:"Commit staged changes."`
	Reset      ResetCmd      `cmd:"" help:"Unstage changes."`
	ApplyPatch ApplyPatchCmd `cmd:"" help:"Apply a patch to the staging area."`
	Version    VersionCmd    `cmd:"" help:"Print the version number."`
}

// Globals carries the process I/O and the repository handle that every
// subcommand needs. Kong injects it into each Run method.
type Globals struct {
	Git  *git.ShellExecutor
	In   io.Reader
	Out  io.Writer
	JSON bool
}

func main() { os.Exit(run()) }

// run binds the process to the CLI and reports its exit status. The signal
// context lives here rather than in main so that its teardown can be
// deferred; an os.Exit alongside it would skip the deferred call.
//
// Cancelling on a signal is what stops a long git command from outliving
// us. The executor turns that cancellation into SIGTERM so git unlinks
// .git/index.lock on its way out; killing it outright would strand the
// lock and wedge the repository.
func run() int {
	ctx, cancel := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer cancel()

	err := execute(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		return 1
	}

	return 0
}

// execute parses argv and runs the selected command. Failures are reported on
// errOut and returned rather than exiting, which keeps the whole CLI — error
// paths included — reachable from tests.
//
// Usage is deliberately never printed alongside an error: a wall of help text
// buried real failures such as "patch does not apply", and callers piping our
// output want a clean error line.
func execute(ctx context.Context, argv []string, r io.Reader, out, errOut io.Writer) error {
	var cli CLI

	parser := newParser(ctx, &cli, out, errOut)

	kctx, err := parser.Parse(argv)
	if err != nil {
		parser.Errorf("%s", err)

		return err
	}

	err = kctx.Run(&Globals{
		Git:  git.NewShellExecutor(cli.Dir),
		In:   r,
		Out:  out,
		JSON: cli.JSON,
	})
	if err != nil {
		parser.Errorf("%s", err)
	}

	return err
}

// newParser builds the command grammar. The writers receive kong's own help
// and error output; a running command writes to Globals.Out instead.
func newParser(ctx context.Context, cli *CLI, out, errOut io.Writer) *kong.Kong {
	return kong.Must(cli,
		kong.Name("git-hunk"),
		kong.Description(description),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Writers(out, errOut),
		kong.BindTo(ctx, (*context.Context)(nil)),
	)
}
