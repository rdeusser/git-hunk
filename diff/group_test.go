package diff_test

import (
	"testing"

	"github.com/rdeusser/git-hunk/diff"
	"github.com/stretchr/testify/require"
)

func groupsOf(t *testing.T, text string) []diff.ChangeGroup {
	t.Helper()

	parsed, err := diff.Parse(text)
	require.NoError(t, err)

	var groups []diff.ChangeGroup
	for group := range parsed.ChangeGroups() {
		groups = append(groups, group)
	}

	return groups
}

func TestChangeGroups(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []diff.ChangeGroup
	}{
		{
			name: "a context line splits one hunk into two groups",
			input: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,4 +1,6 @@
 package main
+import "log"
 func main() {
+	log.Print("hi")
 }
`,
			want: []diff.ChangeGroup{
				{
					Path:    "main.go",
					Start:   2,
					End:     2,
					Added:   1,
					Preview: `import "log"`,
				},
				{
					Path:    "main.go",
					Start:   4,
					End:     4,
					Added:   1,
					Preview: "\tlog.Print(\"hi\")",
				},
			},
		},
		{
			name: "adjacent additions form one group",
			input: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,2 +1,4 @@
 package main
+import "log"
+import "os"
 func main() {}
`,
			want: []diff.ChangeGroup{{
				Path:    "main.go",
				Start:   2,
				End:     3,
				Added:   2,
				Preview: `import "log"`,
			}},
		},
		{
			name: "a replacement is one group and prefers the addition",
			input: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package main
-var old = 1
+var new = 2
 func main() {}
`,
			want: []diff.ChangeGroup{{
				Path:    "main.go",
				Start:   2,
				End:     2,
				Added:   1,
				Deleted: 1,
				Preview: "var new = 2",
			}},
		},
		{
			name: "a deletion-only group is addressed in old-file numbers",
			input: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,2 @@
 package main
-var a = 1
-var b = 2
-var c = 3
 func main() {}
`,
			want: []diff.ChangeGroup{{
				Path:    "main.go",
				Start:   2,
				End:     4,
				Deleted: 3,
				Preview: "var a = 1",
			}},
		},
		{
			// The first added line is blank, so the row would carry no
			// preview at all if the group took its first changed line.
			name: "the preview skips a blank leading addition",
			input: `diff --git a/util.go b/util.go
--- a/util.go
+++ b/util.go
@@ -1,1 +1,3 @@
 func util() {}
+
+func extra() {}
`,
			want: []diff.ChangeGroup{{
				Path:    "util.go",
				Start:   2,
				End:     3,
				Added:   2,
				Preview: "func extra() {}",
			}},
		},
		{
			name: "a group of only blank additions has no preview",
			input: `diff --git a/util.go b/util.go
--- a/util.go
+++ b/util.go
@@ -1,1 +1,3 @@
 func util() {}
+
+
`,
			want: []diff.ChangeGroup{{
				Path:  "util.go",
				Start: 2,
				End:   3,
				Added: 2,
			}},
		},
		{
			name: "a binary file contributes no groups",
			input: `diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, groupsOf(t, tt.input))
		})
	}
}

func TestChangeGroupSelector(t *testing.T) {
	tests := []struct {
		name  string
		group diff.ChangeGroup
		want  string
	}{
		{
			name:  "a single line omits the range",
			group: diff.ChangeGroup{Path: "main.go", Start: 7, End: 7},
			want:  "main.go:7",
		},
		{
			name:  "a span reads as a range",
			group: diff.ChangeGroup{Path: "main.go", Start: 7, End: 9},
			want:  "main.go:7-9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.group.Selector())
		})
	}
}

// TestChangeGroupsStopEarly covers the iterator contract: a caller that
// breaks out of the range loop must not drive the rest of the diff.
func TestChangeGroupsStopEarly(t *testing.T) {
	parsed, err := diff.Parse(`diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,5 @@
 package main
+var a = 1
 func main() {}
+var b = 2
`)
	require.NoError(t, err)

	var seen int

	for range parsed.ChangeGroups() {
		seen++

		break
	}

	require.Equal(t, 1, seen)
}
