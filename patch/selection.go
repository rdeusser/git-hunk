package patch

import (
	"fmt"

	"github.com/rdeusser/git-hunk/diff"
)

// AmbiguousSelectionError reports a selector value that lands on two separate
// changes with nothing in the rest of the selection to say which was meant.
type AmbiguousSelectionError struct {
	// Path is the file the selection named.
	Path string

	// Line is the selector value that landed in two places.
	Line int
}

func (e *AmbiguousSelectionError) Error() string {
	return fmt.Sprintf("%s:%d names two separate changes", e.Path, e.Line)
}

// resolvedSelection records which changed lines a selection names, indexed by
// hunk and then by position within that hunk.
type resolvedSelection [][]bool

// resolveSelection decides which of the file's changed lines the selection
// names.
//
// The decision cannot be made one line at a time. FILE:LINES carries one
// integer namespace while a diff carries two: a deletion is numbered in the
// old file and an addition in the new one. Once earlier additions shift the
// two apart, a single value lands on a deletion in one place and an addition
// in another, and matching each line on its own quietly stages both.
//
// The rest of the selection settles most of these. A value is given to every
// group holding it when the selection covers those groups whole, which is
// what staging a file or a whole listed row does. Otherwise it goes to
// whichever group the selection also reaches through a value that lands
// nowhere else, which is the group the caller plainly meant.
//
// When neither holds, the selection really is undecidable and this reports
// rather than guesses. No rule can do better: the two changes have the same
// address, so a caller who names only that address has said both.
func resolveSelection(file *diff.FileDiff, sel *diff.FileSelection) (resolvedSelection, error) {
	resolved := make(resolvedSelection, len(file.Hunks))
	for i, hunk := range file.Hunks {
		resolved[i] = make([]bool, len(hunk.Lines))
	}

	sites, values := changeSites(file)
	contested := contestedValues(sites, sel)

	covered := coveredGroups(values, sel)
	reached := reachedGroups(sites, sel, contested)

	for _, site := range sites {
		if !sel.Contains(site.value) {
			continue
		}

		groups := contested[site.value]
		if len(groups) < 2 {
			resolved[site.hunk][site.line] = true

			continue
		}

		switch {
		case allCovered(groups, covered):
			// Every group holding the value is selected whole, so
			// the caller asked for all of them.
			resolved[site.hunk][site.line] = true

		case anyReached(groups, reached):
			resolved[site.hunk][site.line] = reached[site.group]

		default:
			return nil, &AmbiguousSelectionError{
				Path: file.Path(),
				Line: site.value,
			}
		}
	}

	return resolved, nil
}

// changeSite is one changed line, located well enough to settle a contested
// selector value.
type changeSite struct {
	hunk  int
	line  int
	value int
	group int
}

// changeSites lists every changed line in the file along with the values each
// change group occupies. A group is a contiguous run of changed lines, which
// cannot span hunks.
func changeSites(file *diff.FileDiff) ([]changeSite, map[int][]int) {
	var (
		sites  []changeSite
		values = make(map[int][]int)
		group  = -1
	)

	for h, hunk := range file.Hunks {
		inGroup := false

		for l, line := range hunk.Lines {
			if !line.IsChange() {
				inGroup = false

				continue
			}

			if !inGroup {
				group++
				inGroup = true
			}

			value := line.EffectiveLineNum()

			sites = append(sites, changeSite{
				hunk:  h,
				line:  l,
				value: value,
				group: group,
			})
			values[group] = append(values[group], value)
		}
	}

	return sites, values
}

// contestedValues maps each selected value to the change groups it lands in.
// Only a value landing in two or more groups is in conflict.
func contestedValues(sites []changeSite, sel *diff.FileSelection) map[int][]int {
	holders := make(map[int]map[int]bool)

	for _, site := range sites {
		if !sel.Contains(site.value) {
			continue
		}

		if holders[site.value] == nil {
			holders[site.value] = make(map[int]bool)
		}

		holders[site.value][site.group] = true
	}

	contested := make(map[int][]int, len(holders))
	for value, groups := range holders {
		for group := range groups {
			contested[value] = append(contested[value], group)
		}
	}

	return contested
}

// coveredGroups reports the groups whose every value the selection names.
func coveredGroups(values map[int][]int, sel *diff.FileSelection) map[int]bool {
	covered := make(map[int]bool, len(values))

	for group, groupValues := range values {
		covered[group] = true

		for _, value := range groupValues {
			if !sel.Contains(value) {
				covered[group] = false

				break
			}
		}
	}

	return covered
}

// reachedGroups reports the groups the selection names through a value that
// lands nowhere else. Such a value is proof the caller meant that group, so it
// can settle a value that is in conflict.
func reachedGroups(sites []changeSite, sel *diff.FileSelection, contested map[int][]int) map[int]bool {
	reached := make(map[int]bool)

	for _, site := range sites {
		if !sel.Contains(site.value) {
			continue
		}

		if len(contested[site.value]) < 2 {
			reached[site.group] = true
		}
	}

	return reached
}

func allCovered(groups []int, covered map[int]bool) bool {
	for _, group := range groups {
		if !covered[group] {
			return false
		}
	}

	return true
}

func anyReached(groups []int, reached map[int]bool) bool {
	for _, group := range groups {
		if reached[group] {
			return true
		}
	}

	return false
}
