// Package git runs git commands against a repository on disk.
package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// terminateGrace is how long a git command gets to clean up after SIGTERM
// before Go kills it outright. Git only has to unlink its lockfiles, so the
// window is generous; nothing waits on it except a cancelled command.
const terminateGrace = 2 * time.Second

// ShellExecutor runs git operations by shelling out to the git binary.
type ShellExecutor struct {
	// WorkDir is the working directory for git commands.
	// If empty, uses current directory.
	WorkDir string
}

// NewShellExecutor creates a new ShellExecutor.
func NewShellExecutor(workDir string) *ShellExecutor {
	return &ShellExecutor{WorkDir: workDir}
}

// run executes a git command and returns stdout.
func (e *ShellExecutor) run(
	ctx context.Context, stdin io.Reader, args ...string,
) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if e.WorkDir != "" {
		cmd.Dir = e.WorkDir
	}

	// CommandContext cancels with SIGKILL, which git cannot handle. Killed
	// between taking .git/index.lock and renaming it over the index, git
	// leaves the lock behind and every later command in the repository
	// fails until someone removes it by hand. SIGTERM runs git's own
	// cleanup instead, and WaitDelay bounds how long that cleanup gets
	// before Go escalates to SIGKILL anyway.
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = terminateGrace

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdin

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(
			"git %s failed: %w: %s",
			strings.Join(args, " "), err, stderr.String(),
		)
	}

	return stdout.String(), nil
}

// Diff returns the unified diff for unstaged changes.
func (e *ShellExecutor) Diff(
	ctx context.Context, paths ...string,
) (string, error) {
	return e.run(ctx, nil, pathArgs([]string{"diff", "--no-color"}, paths)...)
}

// DiffCached returns the unified diff for staged changes.
func (e *ShellExecutor) DiffCached(
	ctx context.Context, paths ...string,
) (string, error) {
	args := []string{"diff", "--cached", "--no-color"}

	return e.run(ctx, nil, pathArgs(args, paths)...)
}

// pathArgs appends paths to args behind a "--" separator. Without it git
// resolves each path as a revision first, so "diff main" fails outright in a
// repository that has both a branch and a file called main, and a path that
// only names a branch silently diffs against that branch instead.
func pathArgs(args, paths []string) []string {
	if len(paths) == 0 {
		return args
	}

	args = append(args, "--")

	return append(args, paths...)
}

// ApplyPatch applies a patch to the staging area.
func (e *ShellExecutor) ApplyPatch(
	ctx context.Context, patch io.Reader,
) error {
	_, err := e.run(ctx, patch, "apply", "--cached", "-")

	return err
}

// Commit creates a commit with the given message.
func (e *ShellExecutor) Commit(ctx context.Context, message string) error {
	_, err := e.run(ctx, nil, "commit", "-m", message)

	return err
}

// Reset unstages all staged changes.
func (e *ShellExecutor) Reset(ctx context.Context) error {
	_, err := e.run(ctx, nil, "reset", "HEAD")

	return err
}

// ResetPaths unstages changes for the given paths in one git invocation, so
// a path git rejects leaves the index untouched rather than half-reset.
func (e *ShellExecutor) ResetPaths(
	ctx context.Context, paths ...string,
) error {
	args := pathArgs([]string{"reset", "HEAD"}, paths)

	_, err := e.run(ctx, nil, args...)

	return err
}

// Status returns the current repository status.
func (e *ShellExecutor) Status(ctx context.Context) (*RepoStatus, error) {
	output, err := e.run(ctx, nil, "status", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}

	return parseStatus(output), nil
}

// parseStatus reads git's porcelain v1 -z output, whose entries are
// "XY <path>\0" with X describing the index and Y the working tree.
//
// A rename or a copy appends a second "<origPath>\0" field to its entry.
// Reading that field as an entry of its own takes its first two bytes for a
// status code and files the rest under a path missing its first three
// characters, so it is skipped explicitly.
func parseStatus(output string) *RepoStatus {
	status := &RepoStatus{}

	fields := strings.Split(output, "\x00")

	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}

		index, worktree, path := entry[0], entry[1], entry[3:]

		if isRenameOrCopy(index) || isRenameOrCopy(worktree) {
			i++
		}

		if index == '?' && worktree == '?' {
			status.UntrackedFiles = append(status.UntrackedFiles, path)

			continue
		}

		// The two sides are independent: "MM" is a file modified in the
		// index and modified again since, and belongs on both lists.
		if index != ' ' {
			status.StagedFiles = append(status.StagedFiles, path)
		}

		if worktree != ' ' {
			status.UnstagedFiles = append(status.UnstagedFiles, path)
		}
	}

	return status
}

func isRenameOrCopy(code byte) bool {
	return code == 'R' || code == 'C'
}

// Root returns the repository root directory.
func (e *ShellExecutor) Root(ctx context.Context) (string, error) {
	output, err := e.run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

// RepoStatus represents the current state of the repository.
type RepoStatus struct {
	// StagedFiles lists files with staged changes.
	StagedFiles []string

	// UnstagedFiles lists files with unstaged changes.
	UnstagedFiles []string

	// UntrackedFiles lists untracked files.
	UntrackedFiles []string
}
