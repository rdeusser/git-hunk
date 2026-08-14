// Package output provides formatting for diff output.
package output

import (
	"encoding/json"
	"io"

	"github.com/rdeusser/git-hunk/diff"
)

// DiffOutput is the top-level JSON output structure.
type DiffOutput struct {
	Files     []FileOutput `json:"files"`
	Untracked []string     `json:"untracked,omitempty"`
}

// FileOutput represents a file in JSON output.
type FileOutput struct {
	Path    string       `json:"path"`
	OldPath string       `json:"old_path,omitempty"`
	Status  string       `json:"status"` // "modified", "new", "deleted", "renamed"
	Binary  bool         `json:"binary,omitempty"`
	Hunks   []HunkOutput `json:"hunks,omitempty"`
}

// HunkOutput represents a hunk in JSON output.
type HunkOutput struct {
	Header  string       `json:"header"`
	Section string       `json:"section,omitempty"`
	Hunks   []LineOutput `json:"lines"`
}

// LineOutput represents a line in JSON output.
type LineOutput struct {
	Op         string `json:"op"` // "add", "delete", "context"
	Content    string `json:"content"`
	OldLineNum int    `json:"old_line,omitempty"`
	NewLineNum int    `json:"new_line,omitempty"`
}

// GroupListOutput is the top-level JSON output for a change-group listing.
type GroupListOutput struct {
	Groups    []GroupOutput `json:"groups"`
	Untracked []string      `json:"untracked,omitempty"`
}

// GroupOutput represents one independently stageable change group.
type GroupOutput struct {
	// Selector is the FILE:LINES argument that stages this group.
	Selector string `json:"selector"`
	Path     string `json:"path"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Added    int    `json:"added"`
	Deleted  int    `json:"deleted"`
	Preview  string `json:"preview"`
}

// FormatJSON writes the parsed diff as JSON.
func FormatJSON(w io.Writer, parsed *diff.ParsedDiff) error {
	return FormatJSONWithUntracked(w, parsed, nil)
}

// FormatJSONWithUntracked writes the parsed diff as JSON, including untracked files.
func FormatJSONWithUntracked(w io.Writer, parsed *diff.ParsedDiff, untracked []string) error {
	output := DiffOutput{
		Files:     make([]FileOutput, 0),
		Untracked: untracked,
	}

	for file := range parsed.Files() {
		fo := FileOutput{
			Path:    file.Path(),
			OldPath: file.OldName,
			Status:  fileStatus(file),
			Binary:  file.IsBinary,
			Hunks:   make([]HunkOutput, 0, len(file.Hunks)),
		}

		if fo.OldPath == fo.Path {
			fo.OldPath = ""
		}

		for _, hunk := range file.Hunks {
			ho := HunkOutput{
				Header:  hunk.Header(),
				Section: hunk.Section,
				Hunks:   make([]LineOutput, 0, len(hunk.Lines)),
			}

			for _, line := range hunk.Lines {
				lo := LineOutput{
					Op:         line.Op.String(),
					Content:    line.Content,
					OldLineNum: line.OldLineNum,
					NewLineNum: line.NewLineNum,
				}
				ho.Hunks = append(ho.Hunks, lo)
			}

			fo.Hunks = append(fo.Hunks, ho)
		}

		output.Files = append(output.Files, fo)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(output)
}

// FormatJSONGroups writes the diff's change groups as JSON.
func FormatJSONGroups(w io.Writer, parsed *diff.ParsedDiff, untracked []string) error {
	output := GroupListOutput{
		Groups:    make([]GroupOutput, 0),
		Untracked: untracked,
	}

	if parsed != nil {
		for group := range parsed.ChangeGroups() {
			output.Groups = append(output.Groups, GroupOutput{
				Selector: group.Selector(),
				Path:     group.Path,
				Start:    group.Start,
				End:      group.End,
				Added:    group.Added,
				Deleted:  group.Deleted,
				Preview:  group.Preview,
			})
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(output)
}

// FormatJSONEmpty writes an empty JSON response.
func FormatJSONEmpty(w io.Writer) error { return FormatJSONEmptyWithUntracked(w, nil) }

// FormatJSONEmptyWithUntracked writes an empty JSON response with untracked files.
func FormatJSONEmptyWithUntracked(w io.Writer, untracked []string) error {
	output := DiffOutput{
		Files:     []FileOutput{},
		Untracked: untracked,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(output)
}

// fileStatus returns the status string for a file.
func fileStatus(f *diff.FileDiff) string {
	switch {
	case f.IsNew:
		return "new"
	case f.IsDeleted:
		return "deleted"
	case f.IsRenamed:
		return "renamed"
	default:
		return "modified"
	}
}
