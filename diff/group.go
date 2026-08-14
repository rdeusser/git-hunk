package diff

import (
	"fmt"
	"iter"
	"strings"
)

// ChangeGroup is a contiguous run of changed lines with no context line
// between them. It is the smallest unit that stages on its own: a group
// mixing additions and deletions is a single replacement, and staging part
// of one would describe a file that never existed.
type ChangeGroup struct {
	// Path is the file the group belongs to, spelled the way a selection
	// names it.
	Path string

	// Start and End bound the group in the numbering a selection matches
	// against, which is the new file for additions and the old file for
	// deletions. See DiffLine.EffectiveLineNum.
	Start int
	End   int

	// Added and Deleted count the lines the group contributes.
	Added   int
	Deleted int

	// Preview is the unmodified content of the line that best identifies
	// the group: its first non-blank addition, or its first non-blank
	// deletion when the group only removes lines.
	Preview string
}

// Selector returns the FILE:LINES argument that stages this group.
func (g ChangeGroup) Selector() string {
	if g.Start == g.End {
		return fmt.Sprintf("%s:%d", g.Path, g.Start)
	}

	return fmt.Sprintf("%s:%d-%d", g.Path, g.Start, g.End)
}

// ChangeGroups iterates every contiguous run of changed lines in the diff,
// file by file and hunk by hunk. A file with no hunks yields nothing, so a
// binary change contributes no groups: it has no lines to stage.
func (d *ParsedDiff) ChangeGroups() iter.Seq[ChangeGroup] {
	return func(yield func(ChangeGroup) bool) {
		for _, file := range d.files {
			for _, hunk := range file.Hunks {
				for _, group := range groupChanges(hunk, file.Path()) {
					if !yield(group) {
						return
					}
				}
			}
		}
	}
}

// groupBuilder accumulates one run of changed lines. It keeps the addition
// and deletion previews apart so the finished group can prefer whichever
// describes the change better.
type groupBuilder struct {
	start      int
	end        int
	added      int
	deleted    int
	addPreview string
	delPreview string
}

// preview picks the line that best identifies the group. An addition says
// what the change produces, which is what the caller is deciding whether to
// stage, so it wins over the deletion it replaces. Blank lines identify
// nothing and are skipped on the way in.
func (b *groupBuilder) preview() string {
	if b.addPreview != "" {
		return b.addPreview
	}

	return b.delPreview
}

// groupChanges splits a hunk into its contiguous runs of changed lines. A
// context line closes the run it follows, which is what leaves each returned
// group independently stageable.
func groupChanges(hunk *Hunk, path string) []ChangeGroup {
	var (
		groups  []ChangeGroup
		current *groupBuilder
	)

	closeGroup := func() {
		if current == nil {
			return
		}

		groups = append(groups, ChangeGroup{
			Path:    path,
			Start:   current.start,
			End:     current.end,
			Added:   current.added,
			Deleted: current.deleted,
			Preview: current.preview(),
		})

		current = nil
	}

	for _, line := range hunk.Lines {
		if !line.IsChange() {
			closeGroup()

			continue
		}

		num := line.EffectiveLineNum()

		if current == nil {
			current = &groupBuilder{start: num, end: num}
		}

		// A replacement lists its deletions before its additions, so the
		// numbers arrive out of order: the old-file line of a deletion can
		// sit above the new-file line of the addition that replaces it.
		if num < current.start {
			current.start = num
		}

		if num > current.end {
			current.end = num
		}

		blank := strings.TrimSpace(line.Content) == ""

		if line.Op == OpAdd {
			current.added++

			if current.addPreview == "" && !blank {
				current.addPreview = line.Content
			}

			continue
		}

		current.deleted++

		if current.delPreview == "" && !blank {
			current.delPreview = line.Content
		}
	}

	closeGroup()

	return groups
}
