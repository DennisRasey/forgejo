// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package gitdiff

import (
	"strings"
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitLines(t *testing.T) {
	assert.Nil(t, splitLines(""))
	assert.Equal(t, []string{"a"}, splitLines("a"))
	assert.Equal(t, []string{"a"}, splitLines("a\n"))
	assert.Equal(t, []string{"a", "b"}, splitLines("a\nb\n"))
	assert.Equal(t, []string{"a", "", "b"}, splitLines("a\n\nb\n"))
	assert.Equal(t, []string{"a", ""}, splitLines("a\n\n"))
	// CRLF is normalized so head "before" lines match the LF-normalized suggestion
	assert.Equal(t, []string{"a", "b"}, splitLines("a\r\nb\r\n"))
	assert.Equal(t, []string{"a", "b"}, splitLines("a\r\nb"))
}

func countLineTypes(diff *Diff) (dels, adds int) {
	for _, file := range diff.Files {
		for _, section := range file.Sections {
			for _, line := range section.Lines {
				switch line.Type {
				case DiffLineDel:
					dels++
				case DiffLineAdd:
					adds++
				}
			}
		}
	}
	return dels, adds
}

func parseSuggestion(t *testing.T, treePath string, startLine uint64, original, suggestion []string) *Diff {
	t.Helper()
	patch := synthesizeSuggestionPatch(treePath, startLine, original, suggestion)
	diff, err := ParsePatch(db.DefaultContext, setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, strings.NewReader(patch))
	require.NoError(t, err)
	return diff
}

func TestSynthesizeSuggestionPatch(t *testing.T) {
	assert.Equal(t,
		"diff --git a/README.md b/README.md\n--- a/README.md\n+++ b/README.md\n@@ -3,2 +3,1 @@\n-old a\n-old b\n+new\n",
		synthesizeSuggestionPatch("README.md", 3, []string{"old a", "old b"}, []string{"new"}))

	diff := parseSuggestion(t, "README.md", 3, []string{"old a", "old b"}, []string{"new"})
	require.Len(t, diff.Files, 1)
	assert.Equal(t, "README.md", diff.Files[0].Name)
	dels, adds := countLineTypes(diff)
	assert.Equal(t, 2, dels)
	assert.Equal(t, 1, adds)
}

func TestSynthesizeSuggestionPatchDelete(t *testing.T) {
	// An empty suggestion deletes the anchored range (no "+" lines, new-side count is 0).
	assert.Equal(t,
		"diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n@@ -1,1 +1,0 @@\n-gone\n",
		synthesizeSuggestionPatch("f.txt", 1, []string{"gone"}, nil))

	diff := parseSuggestion(t, "f.txt", 1, []string{"gone"}, nil)
	require.Len(t, diff.Files, 1)
	dels, adds := countLineTypes(diff)
	assert.Equal(t, 1, dels)
	assert.Equal(t, 0, adds)
}

func TestSynthesizeSuggestionPatchMultiToMulti(t *testing.T) {
	assert.Equal(t,
		"diff --git a/dir/file.go b/dir/file.go\n--- a/dir/file.go\n+++ b/dir/file.go\n@@ -10,3 +10,2 @@\n-a\n-b\n-c\n+x\n+y\n",
		synthesizeSuggestionPatch("dir/file.go", 10, []string{"a", "b", "c"}, []string{"x", "y"}))

	diff := parseSuggestion(t, "dir/file.go", 10, []string{"a", "b", "c"}, []string{"x", "y"})
	require.Len(t, diff.Files, 1)
	assert.Equal(t, "dir/file.go", diff.Files[0].Name)
	dels, adds := countLineTypes(diff)
	assert.Equal(t, 3, dels)
	assert.Equal(t, 2, adds)
}
