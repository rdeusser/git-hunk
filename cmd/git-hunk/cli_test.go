package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "commands-test-*")
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(dir)
	}

	// Initialize git repo.
	gitCmd(t, dir, "init")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test User")

	return dir, cleanup
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()

	if args[0] == "init" {
		args = append([]string{"-c", "init.defaultBranch=main"}, args...)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %v failed: %v\n%s", args, err, out)
	}

	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

// runCLI runs the CLI the way main does and returns everything it wrote.
func runCLI(t *testing.T, argv ...string) (string, error) {
	t.Helper()

	var output bytes.Buffer

	err := run(
		context.Background(), argv,
		strings.NewReader(""), &output, &output,
	)

	return output.String(), err
}

func TestGrammar(t *testing.T) {
	var cli CLI

	parser := newParser(context.Background(), &cli, io.Discard, io.Discard)
	require.Equal(t, "git-hunk", parser.Model.Name)

	commands := map[string]bool{
		"diff":        true,
		"stage":       true,
		"preview":     true,
		"commit":      true,
		"reset":       true,
		"apply-patch": true,
		"version":     false,
	}

	for _, node := range parser.Model.Children {
		wantDetail, known := commands[node.Name]
		require.True(t, known, "unexpected command %q", node.Name)
		require.NotEmpty(t, node.Help, "%s needs help text", node.Name)

		if wantDetail {
			require.NotEmpty(t, node.Detail,
				"%s needs a Help() detail block", node.Name)
		}

		delete(commands, node.Name)
	}

	require.Empty(t, commands, "commands missing from the grammar")
}

func TestDiffCommandExecution(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit a file.
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Make changes.
	writeFile(t, dir, "main.go", "package main\n\n// Added.\nfunc main() {}\n")

	// Create the command and run it.
	output, err := runCLI(t, "--dir", dir, "diff")
	require.NoError(t, err)

	require.Contains(t, output, "+// Added.")
}

func TestDiffCommandJSON(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit a file.
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Make changes.
	writeFile(t, dir, "main.go", "package main\n\n// Added.\nfunc main() {}\n")

	// Run with JSON flag.
	output, err := runCLI(t, "--dir", dir, "--json", "diff")
	require.NoError(t, err)

	require.Contains(t, output, "\"files\"")
}

func TestPreviewCommandEmpty(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit a file so we have a valid repo.
	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	output, err := runCLI(t, "--dir", dir, "preview")
	require.NoError(t, err)

	require.Contains(t, output, "Nothing staged")
}

func TestResetCommand(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit a file.
	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Stage changes.
	writeFile(t, dir, "main.go", "package main\n// changed\n")
	gitCmd(t, dir, "add", "main.go")

	output, err := runCLI(t, "--dir", dir, "reset")
	require.NoError(t, err)

	require.Contains(t, output, "Unstaged")
}

func TestStageCommandInvalidSelection(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	_, err := runCLI(t, "--dir", dir, "stage", "invalid")
	require.Error(t, err)
}

func TestCommitCommandNoMessage(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")

	_, err := runCLI(t, "--dir", dir, "commit")
	require.Error(t, err)
}

func TestDiffCommandNoChanges(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit a file - no uncommitted changes.
	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	_, err := runCLI(t, "--dir", dir, "diff")
	require.NoError(t, err)
	// Empty diff should succeed without output.
}

func TestApplyPatchCommandNoFile(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Try to apply non-existent file.
	_, err := runCLI(t, "--dir", dir, "apply-patch", "nonexistent.patch")
	require.Error(t, err)
}

func TestDiffCommandStaged(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Stage some changes.
	writeFile(t, dir, "main.go", "package main\n// staged\n")
	gitCmd(t, dir, "add", "main.go")

	output, err := runCLI(t, "--dir", dir, "diff", "--staged")
	require.NoError(t, err)

	require.Contains(t, output, "+// staged")
}

func TestStageCommandNoChanges(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// No unstaged changes.
	_, err := runCLI(t, "--dir", dir, "stage", "main.go:1-10")
	require.Error(t, err) // Should error: no unstaged changes.
}

func TestCommitCommandNothingStaged(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Nothing staged.
	_, err := runCLI(t, "--dir", dir, "commit", "-m", "test")
	require.Error(t, err) // Should error: nothing staged.
}

func TestDiffCommandFlags(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")
	writeFile(t, dir, "main.go", "package main\n// changed\n")

	// Test --files flag - just verify it doesn't error.
	_, err := runCLI(t, "--dir", dir, "diff", "--files")
	require.NoError(t, err)
}

func TestPreviewCommandRaw(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Stage changes.
	writeFile(t, dir, "main.go", "package main\n// staged\n")
	gitCmd(t, dir, "add", "main.go")

	// Just verify no error.
	_, err := runCLI(t, "--dir", dir, "preview", "--raw")
	require.NoError(t, err)
}

func TestDiffCommandSummary(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")
	writeFile(t, dir, "main.go", "package main\n// changed\n")

	_, err := runCLI(t, "--dir", dir, "diff", "--summary")
	require.NoError(t, err)
}

func TestDiffCommandStageHints(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")
	writeFile(t, dir, "main.go", "package main\n// changed\n")

	_, err := runCLI(t, "--dir", dir, "diff", "--stage-hints")
	require.NoError(t, err)
}

func TestDiffCommandRaw(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")
	writeFile(t, dir, "main.go", "package main\n// changed\n")

	_, err := runCLI(t, "--dir", dir, "diff", "--raw")
	require.NoError(t, err)
}

// TestStageAtomicReplacementGroup verifies that staging a partial selection
// of a replacement group (mixed deletions + additions) includes the entire
// group. This is the integration test for the "atomic change group" fix
// that prevents "patch does not apply" errors when a user's line range
// boundary falls in the middle of a contiguous replacement.
func TestStageAtomicReplacementGroup(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Original file with multiple functions.
	original := `package main

func oldHelper1() {}
func oldHelper2() {}
func oldHelper3() {}
func oldHelper4() {}

func main() {}
`
	writeFile(t, dir, "main.go", original)
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modified file: replace the 4 old helpers with 2 new ones.
	modified := `package main

func newHelper1() {}
func newHelper2() {}

func main() {}
`
	writeFile(t, dir, "main.go", modified)

	// Stage only new line 3 (first addition). The replacement group
	// includes old lines 3-6 (deletions) and new lines 3-4 (additions).
	// Without the atomic group fix, only the addition at line 3 would
	// be staged, creating an invalid patch.
	_, err := runCLI(t, "--dir", dir, "stage", "main.go:3")
	require.NoError(t, err, "stage should succeed for partial "+
		"replacement selection")

	// Verify the staged diff includes all deletions and additions.
	cached := gitCmd(t, dir, "diff", "--cached")
	require.Contains(t, cached, "-func oldHelper1()")
	require.Contains(t, cached, "-func oldHelper2()")
	require.Contains(t, cached, "-func oldHelper3()")
	require.Contains(t, cached, "-func oldHelper4()")
	require.Contains(t, cached, "+func newHelper1()")
	require.Contains(t, cached, "+func newHelper2()")
}

// TestStageMultiHunkReplacementBoundary tests the real-world scenario where
// a non-contiguous selection spans multiple hunks and a range boundary falls
// inside a replacement group in one of the hunks.
func TestStageMultiHunkReplacementBoundary(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Original file with two sections separated by enough context
	// to create separate hunks.
	original := `package main

// Section A.
func a1() {}
func a2() {}

// Separator line 1.
// Separator line 2.
// Separator line 3.
// Separator line 4.
// Separator line 5.

// Section B.
func b1() {}
func b2() {}
func b3() {}

func main() {}
`
	writeFile(t, dir, "main.go", original)
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Replace both sections.
	modified := `package main

// Section A.
func newA() {}

// Separator line 1.
// Separator line 2.
// Separator line 3.
// Separator line 4.
// Separator line 5.

// Section B.
func newB() {}

func main() {}
`
	writeFile(t, dir, "main.go", modified)

	// Stage only section A changes (new line 4). This should pick up
	// both the deletions (a1, a2) and the addition (newA) in hunk 1,
	// but NOT the section B changes in hunk 2.
	_, err := runCLI(t, "--dir", dir, "stage", "main.go:4")
	require.NoError(t, err, "multi-hunk partial staging should succeed")

	cached := gitCmd(t, dir, "diff", "--cached")
	// Section A changes should be staged.
	require.Contains(t, cached, "-func a1()")
	require.Contains(t, cached, "-func a2()")
	require.Contains(t, cached, "+func newA()")

	// Section B changes should NOT be staged (different hunk).
	require.NotContains(t, cached, "-func b1()")
	require.NotContains(t, cached, "+func newB()")
}

// TestStageOverlappingContextGroups is the end-to-end regression test for the
// v1.0.2 report "partial-hunk staging emits non-applying patches". Two mixed
// replacement groups inside a single hunk are separated by one shared context
// line. Selecting both groups used to emit two sub-hunks that each claimed
// that shared line — one as trailing context, one as leading context — so
// their old-side ranges overlapped and `git apply --cached` rejected the
// patch with "patch does not apply". The patch package now coalesces blocks
// whose context windows overlap, emitting the shared context exactly once.
func TestStageOverlappingContextGroups(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Two replacement groups (delA/delB -> addA1..3, delC/delD ->
	// addC1/addC2) straddle a single preserved context line, keepMid.
	original := `package main

func top() {}
// delA.
// delB.
// keepMid.
// delC.
// delD.
func bottom() {}
`
	writeFile(t, dir, "main.go", original)
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	modified := `package main

func top() {}
// addA1.
// addA2.
// addA3.
// keepMid.
// addC1.
// addC2.
func bottom() {}
`
	writeFile(t, dir, "main.go", modified)

	// New line 4 hits the first group; new line 8 hits the second. The
	// selection spans the shared keepMid context line.
	_, err := runCLI(t, "--dir", dir, "stage", "main.go:4,8")
	require.NoError(
		t, err, "staging both groups across a shared context line "+
			"should produce an applying patch",
	)

	// Both replacement groups landed in the index.
	cached := gitCmd(t, dir, "diff", "--cached")
	require.Contains(t, cached, "-// delA.")
	require.Contains(t, cached, "-// delD.")
	require.Contains(t, cached, "+// addA1.")
	require.Contains(t, cached, "+// addC2.")

	// keepMid straddles both groups but is unchanged, so it must not be
	// staged as a modification. The shared context line survives.
	require.NotContains(t, cached, "-// keepMid.")
	require.NotContains(t, cached, "+// keepMid.")

	// The whole change was selected, so nothing modified remains in the
	// working tree relative to the index.
	require.Empty(t, gitCmd(t, dir, "diff", "main.go"))
}

// TestStageNoNewlineAtEOF is the end-to-end regression test for files without
// a trailing newline. git records the "\ No newline at end of file" marker,
// but the go-diff parser strips it; hunk recovers it from the raw diff and
// re-emits it. Without that, any patch whose context or change reaches the
// newline-less final line presents a phantom trailing newline and
// `git apply --cached` rejects it with "patch does not apply".
func TestStageNoNewlineAtEOF(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Commit a file with NO trailing newline on its last line.
	writeFile(t, dir, "main.go", "a\nb\nc")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Insert X near the top and rewrite the newline-less last line.
	writeFile(t, dir, "main.go", "a\nX\nb\nc_mod")

	t.Run("stage insert above newline-less EOF", func(t *testing.T) {
		defer gitCmd(t, dir, "reset", "-q")

		_, err := runCLI(t, "--dir", dir, "stage", "main.go:2")
		require.NoError(t, err)

		// Only X is staged; the final line stays `c` with no newline.
		require.Equal(
			t, "a\nX\nb\nc", gitCmd(t, dir, "show", ":main.go"),
		)
	})

	t.Run("stage the newline-less replacement", func(t *testing.T) {
		defer gitCmd(t, dir, "reset", "-q")

		_, err := runCLI(t, "--dir", dir, "stage", "main.go:4")
		require.NoError(t, err)

		// c -> c_mod staged, still no trailing newline.
		require.Equal(
			t, "a\nb\nc_mod", gitCmd(t, dir, "show", ":main.go"),
		)
	})
}

// TestStageDeletedFile is the end-to-end regression test for staging a file
// deletion. file.NewName is the literal string "/dev/null" for a deleted
// file, so unconditionally writing "+++ b/%s" produced "+++ b//dev/null" —
// git apply --cached rejected every attempt to stage a deletion with
// "error: /dev/null: does not exist in index", regardless of selection.
func TestStageDeletedFile(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "del.txt", "d1\nd2\nd3\nd4\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	t.Run("staging every deletion line removes the file", func(t *testing.T) {
		require.NoError(t, os.Remove(filepath.Join(dir, "del.txt")))
		defer func() {
			gitCmd(t, dir, "reset", "-q")
			gitCmd(t, dir, "checkout", "--", "del.txt")
		}()

		_, err := runCLI(t, "--dir", dir, "stage", "del.txt:1-4")
		require.NoError(t, err)

		status := gitCmd(t, dir, "status", "--short")
		require.Contains(t, status, "D  del.txt")
	})

	t.Run("staging a subset leaves the file modified", func(t *testing.T) {
		require.NoError(t, os.Remove(filepath.Join(dir, "del.txt")))
		defer func() {
			gitCmd(t, dir, "reset", "-q")
			gitCmd(t, dir, "checkout", "--", "del.txt")
		}()

		_, err := runCLI(t, "--dir", dir, "stage", "del.txt:2")
		require.NoError(t, err)

		require.Equal(
			t, "d1\nd3\nd4\n", gitCmd(t, dir, "show", ":del.txt"),
		)
	})
}

// TestStageNewFile is the end-to-end regression test for staging a
// newly-added (intent-to-add) file. This path already applied despite the
// malformed "--- a//dev/null" header, because a pure-addition hunk (@@ -0,0
// ...@@) has nothing on the old side for git apply to match; the header fix
// still removes the malformed path for correctness and robustness.
func TestStageNewFile(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "seed.txt", "seed\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	t.Run("partial selection stages only those lines", func(t *testing.T) {
		writeFile(t, dir, "new.txt", "n1\nn2\nn3\nn4\nn5\n")
		gitCmd(t, dir, "add", "-N", "new.txt")
		defer func() {
			gitCmd(t, dir, "reset", "-q", "--", "new.txt")
			require.NoError(
				t, os.Remove(filepath.Join(dir, "new.txt")),
			)
		}()

		_, err := runCLI(t, "--dir", dir, "stage", "new.txt:2-3")
		require.NoError(t, err)

		require.Equal(
			t, "n2\nn3\n", gitCmd(t, dir, "show", ":new.txt"),
		)
	})

	t.Run("full selection stages the whole file", func(t *testing.T) {
		writeFile(t, dir, "new.txt", "n1\nn2\nn3\nn4\nn5\n")
		gitCmd(t, dir, "add", "-N", "new.txt")
		defer func() {
			gitCmd(t, dir, "reset", "-q", "--", "new.txt")
			require.NoError(
				t, os.Remove(filepath.Join(dir, "new.txt")),
			)
		}()

		_, err := runCLI(t, "--dir", dir, "stage", "new.txt:1-5")
		require.NoError(t, err)

		require.Equal(
			t, "n1\nn2\nn3\nn4\nn5\n", gitCmd(t, dir, "show", ":new.txt"),
		)
	})
}

// TestStagePureAddSubrangeInsideStruct is the end-to-end regression test
// for the silent-misplacement bug originally reported against hunk: when
// new fields are inserted inside an existing Go struct and the user stages
// only a sub-range of the new fields, the staged content must land inside
// the struct body, NOT appended at EOF after `func Helper()`.
//
// The bug shape: a pure-addition group with no context lines between the
// selected sub-range and the surrounding unselected adds emitted a hunk
// with no anchor, which `git apply --cached` silently fuzzed onto the file
// boundary. The patch package now coalesces selections within the same
// pure-add group and walks past unselected additions to grab real anchor
// context, producing a hunk that places the selected lines exactly where
// the user expected them.
func TestStagePureAddSubrangeInsideStruct(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	original := `package foo

type S struct {
	Field1 int
}

func Helper() {}
`
	writeFile(t, dir, "test.go", original)
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Insert five new fields inside the struct body. The unified diff
	// will show a single pure-add group from new lines 5..9.
	modified := `package foo

type S struct {
	Field1 int
	Field2 int
	Field3 int
	Field4 int
	Field5 int
	Field6 int
}

func Helper() {}
`
	writeFile(t, dir, "test.go", modified)

	// Stage just the middle two — Field3 and Field4 — exactly the
	// shape of the original bug report. Without the fix this either
	// errored out with "patch does not apply" or silently placed the
	// fields after `func Helper()`.
	_, err := runCLI(t, "--dir", dir, "stage", "test.go:6-7")
	require.NoError(t, err,
		"pure-add subrange staging must not error",
	)

	// The staged version of the file must contain Field3 and Field4
	// inside the struct, in the original Field-N ordering. The
	// unselected fields must NOT be staged.
	staged := gitCmd(t, dir, "show", ":test.go")
	expected := `package foo

type S struct {
	Field1 int
	Field3 int
	Field4 int
}

func Helper() {}
`
	require.Equal(t, expected, staged,
		"staged content must place selected fields inside the "+
			"struct, not at EOF",
	)
}

// TestStageNonContiguousAddsInSameGroup is the regression test for the
// secondary symptom: when two non-adjacent selections target lines in the
// SAME pure-add group, the two selections must coalesce into one hunk
// with proper anchor context on both sides. Otherwise the second hunk
// would lack a leading anchor and `git apply` would silently misplace it.
func TestStageNonContiguousAddsInSameGroup(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	original := `package foo

type S struct {
	Field1 int
}

func Helper() {}
`
	writeFile(t, dir, "test.go", original)
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	modified := fiveFieldStructFile
	writeFile(t, dir, "test.go", modified)

	// Skip Field3 and Field4 — stage only the first and last new
	// fields. With the pre-fix code these were two separate blocks
	// neither of which had complete anchor context.
	_, err := runCLI(t, "--dir", dir, "stage", "test.go:5,8")
	require.NoError(t, err,
		"non-contiguous staging within same pure-add group must "+
			"succeed",
	)

	staged := gitCmd(t, dir, "show", ":test.go")
	expected := `package foo

type S struct {
	Field1 int
	Field2 int
	Field5 int
}

func Helper() {}
`
	require.Equal(t, expected, staged,
		"non-contiguous selection must land adjacently inside the "+
			"struct in selection order",
	)
}

// fiveFieldStructFile is a shared text fixture: a Go file with a struct
// holding five sequentially-named fields. Several pure-add and
// pure-delete subrange tests share this exact shape, so we extract it
// here to avoid stringly-duplicated literals.
const fiveFieldStructFile = `package foo

type S struct {
	Field1 int
	Field2 int
	Field3 int
	Field4 int
	Field5 int
}

func Helper() {}
`

// TestStagePureDeleteSubrangeInsideStruct is the symmetric regression
// test for pure-delete groups: when the user stages a sub-range of
// deletions inside a larger pure-delete group, the patch must correctly
// describe the unselected deletions as context lines so git apply
// accepts it and the staged file retains exactly the unselected lines.
// Without the fix, the unselected deletion wedged between two selected
// ones was dropped from the patch body and git apply rejected the patch
// with "patch does not apply".
func TestStagePureDeleteSubrangeInsideStruct(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	original := fiveFieldStructFile
	writeFile(t, dir, "test.go", original)
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Remove Field2..Field4. Diff has one pure-delete group of 3 lines.
	modified := `package foo

type S struct {
	Field1 int
	Field5 int
}

func Helper() {}
`
	writeFile(t, dir, "test.go", modified)

	// Stage only Field2 and Field4 (old lines 5 and 7), keeping Field3
	// (old line 6) un-staged inside the group. This is the pure-delete
	// analogue of TestStageNonContiguousAddsInSameGroup.
	_, err := runCLI(t, "--dir", dir, "stage", "test.go:5,7")
	require.NoError(t, err,
		"non-contiguous staging within same pure-delete group "+
			"must succeed",
	)

	staged := gitCmd(t, dir, "show", ":test.go")
	expected := `package foo

type S struct {
	Field1 int
	Field3 int
	Field5 int
}

func Helper() {}
`
	require.Equal(t, expected, staged,
		"pure-delete subrange must remove selected fields and "+
			"preserve the unselected one in between",
	)
}

// TestStagePureDeleteMiddleOnlySingle covers the single-selection-in-
// the-middle-of-a-pure-delete-group case. Selecting only the middle of
// `-B,-C,-D` pre-fix emitted a hunk with no anchor context on either
// side (both neighbours are unselected deletions). With unselected
// deletions now re-tagged as context, the hunk picks up `-B` and `-D`
// as context anchors and the patch applies cleanly.
func TestStagePureDeleteMiddleOnlySingle(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	original := fiveFieldStructFile
	writeFile(t, dir, "test.go", original)
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	modified := `package foo

type S struct {
	Field1 int
	Field5 int
}

func Helper() {}
`
	writeFile(t, dir, "test.go", modified)

	// Stage only Field3 (old line 6) — the middle deletion.
	_, err := runCLI(t, "--dir", dir, "stage", "test.go:6")
	require.NoError(t, err,
		"single-selection inside a pure-delete group must succeed",
	)

	staged := gitCmd(t, dir, "show", ":test.go")
	expected := `package foo

type S struct {
	Field1 int
	Field2 int
	Field4 int
	Field5 int
}

func Helper() {}
`
	require.Equal(t, expected, staged,
		"only the selected deletion must be applied; the "+
			"surrounding unselected deletions stay in place",
	)
}

// TestStageErrorDoesNotPrintUsage verifies that when stage fails (e.g. the
// selection doesn't match any line), the CLI does NOT dump its help text.
// A help dump makes real failures invisible to scripts and AI agents piping
// stdout and stderr.
func TestStageErrorDoesNotPrintUsage(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "main.go", "package main\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Now make a change so there's a diff to operate on.
	writeFile(t, dir, "main.go", "package main\n// added\n")

	// Select a line number with no matching change.
	output, err := runCLI(t, "--dir", dir, "stage", "main.go:9999")
	require.Error(t, err,
		"selection with no matching lines must return an error",
	)

	// "Usage:" and "Examples:" head the two halves of the help block.
	// Neither should appear next to the error.
	require.NotContains(t, output, "Usage:",
		"stage error must not print usage block",
	)
	require.NotContains(t, output, "Examples:",
		"stage error must not print examples block",
	)
}

// TestStageRenamedFile covers a rename that also carries edits. The patch
// must name both paths in a "diff --git" line and repeat them as rename
// from/to, or git apply resolves it against the new path — which does not
// hold the old content — and rejects the whole patch.
func TestStageRenamedFile(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "old.txt", "a\nb\nc\nd\ne\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	require.NoError(t, os.Remove(filepath.Join(dir, "old.txt")))
	writeFile(t, dir, "new.txt", "a\nB2\nc\nD2\ne\n")
	gitCmd(t, dir, "add", "-N", "new.txt")

	_, err := runCLI(t, "--dir", dir, "stage", "new.txt:2")
	require.NoError(t, err, "staging from a renamed file must succeed")

	require.Contains(t, gitCmd(t, dir, "diff", "--cached", "--summary"),
		"rename old.txt => new.txt",
		"the rename itself must be staged",
	)

	require.Equal(t, "a\nB2\nc\nd\ne\n",
		gitCmd(t, dir, "show", ":new.txt"),
		"only the selected line may be staged",
	)
}

// TestStageModeChange covers the exec bit. Git reports a mode change only in
// the extended header block, so a patch built from hunk bodies alone leaves
// the mode behind.
func TestStageModeChange(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "s.sh", "#!/bin/sh\necho a\necho b\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	path := filepath.Join(dir, "s.sh")
	require.NoError(t, os.Chmod(path, 0o755))
	writeFile(t, dir, "s.sh", "#!/bin/sh\necho A2\necho b\n")

	_, err := runCLI(t, "--dir", dir, "stage", "s.sh:2")
	require.NoError(t, err)

	require.Contains(t, gitCmd(t, dir, "ls-files", "-s", "s.sh"), "100755",
		"the execute bit must reach the index",
	)
}

// TestStageRenameWithModeChange covers both extended headers at once, in the
// order git writes them: mode lines before rename lines.
func TestStageRenameWithModeChange(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "old.sh", "#!/bin/sh\necho a\necho b\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	require.NoError(t, os.Remove(filepath.Join(dir, "old.sh")))
	writeFile(t, dir, "new.sh", "#!/bin/sh\necho A2\necho b\n")
	require.NoError(t, os.Chmod(filepath.Join(dir, "new.sh"), 0o755))
	gitCmd(t, dir, "add", "-N", "new.sh")

	_, err := runCLI(t, "--dir", dir, "stage", "new.sh:2")
	require.NoError(t, err)

	require.Contains(t, gitCmd(t, dir, "diff", "--cached", "--summary"),
		"rename old.sh => new.sh",
	)
	require.Contains(t, gitCmd(t, dir, "ls-files", "-s", "new.sh"), "100755")
}

// TestStageRenameDoesNotClaimAnotherFilesPath covers a selection that two
// file diffs can answer to: a.txt renamed away to c.txt, and a new a.txt
// created at the vacated path. Resolving a selection by a rename's old name
// unconditionally puts both in reach of "a.txt:2", and the patch stages
// lines from c.txt that the caller never named.
func TestStageRenameDoesNotClaimAnotherFilesPath(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	var original strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&original, "orig line %d\n", i)
	}

	writeFile(t, dir, "a.txt", original.String())
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Move a.txt to c.txt with an edit, then put a different file back at
	// the path a.txt just vacated.
	moved := strings.Replace(
		original.String(), "orig line 2\n", "RENAMED FILE EDIT\n", 1,
	)
	require.NoError(t, os.Remove(filepath.Join(dir, "a.txt")))
	writeFile(t, dir, "c.txt", moved)
	writeFile(t, dir, "a.txt", "brand new 1\nNEW FILE LINE 2\nbrand new 3\n")
	gitCmd(t, dir, "add", "-N", "c.txt", "a.txt")

	output, err := runCLI(t, "--dir", dir, "stage", "--dry-run", "a.txt:2")
	require.NoError(t, err)

	require.NotContains(t, output, "c.txt",
		"a selection naming a.txt must not reach the renamed file",
	)
	require.NotContains(t, output, "RENAMED FILE EDIT",
		"no line from the renamed file may be staged",
	)
	require.Contains(t, output, "NEW FILE LINE 2")
}

// TestStageRenamedFileByOldName keeps the convenience the guard above must
// not cost: while no other file holds the vacated path, a renamed file is
// still selectable by the name it moved from.
func TestStageRenamedFileByOldName(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	writeFile(t, dir, "old.txt", "a\nb\nc\nd\ne\n")
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-m", "initial")

	require.NoError(t, os.Remove(filepath.Join(dir, "old.txt")))
	writeFile(t, dir, "new.txt", "a\nB2\nc\nD2\ne\n")
	gitCmd(t, dir, "add", "-N", "new.txt")

	_, err := runCLI(t, "--dir", dir, "stage", "old.txt:2")
	require.NoError(t, err)

	require.Equal(t, "a\nB2\nc\nd\ne\n", gitCmd(t, dir, "show", ":new.txt"))
}
