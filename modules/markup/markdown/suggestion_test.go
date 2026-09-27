// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSuggestions(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		expected []string
	}{
		{"no code", "just some text, nothing to see here", nil},
		{"non-suggestion fence", "```go\nfmt.Println()\n```\n", nil},
		{"single", "before\n\n```suggestion\n    return bar()\n```\n\nafter", []string{"    return bar()\n"}},
		{"multi-line", "```suggestion\nline a\nline b\n```", []string{"line a\nline b\n"}},
		{"multiple blocks", "```suggestion\nfoo\n```\ntext\n```suggestion\nbar\n```", []string{"foo\n", "bar\n"}},
		{"mixed with other fences", "```go\nx := 1\n```\n```suggestion\ny := 2\n```", []string{"y := 2\n"}},
		{"empty is delete", "```suggestion\n```", []string{""}},
		{"mixed case keyword", "```Suggestion\nx\n```", []string{"x\n"}},
		{"comma suffix", "```suggestion,extra\nx\n```", []string{"x\n"}},
		{"tilde fence", "~~~suggestion\nx\n~~~", []string{"x\n"}},
		{"indentation and tabs preserved", "```suggestion\n\tindented\n    spaces\n```", []string{"\tindented\n    spaces\n"}},
		{"crlf normalized to lf", "```suggestion\r\na\r\nb\r\n```", []string{"a\nb\n"}},
		{"pandoc brace is not a suggestion", "```{.suggestion}\nx\n```", nil},
		{"blank line inside block", "```suggestion\na\n\nb\n```", []string{"a\n\nb\n"}},
		// a tab in the info-string is kept by the renderer (its language class is then dropped by the
		// sanitizer), so the block renders as plain code; extraction must agree and not count it.
		{"tab in info-string is not a suggestion", "```suggestion\tfoo\nx\n```", nil},
		{"tab block ignored, plain suggestion still extracted", "```suggestion\tfoo\nA\n```\ntext\n```suggestion\nB\n```", []string{"B\n"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ExtractSuggestions(tc.content))
		})
	}
}

func TestIsSuggestionLanguage(t *testing.T) {
	yes := []string{"suggestion", "Suggestion", "SUGGESTION", "suggestion,foo", "suggestion bar", " suggestion "}
	no := []string{"", "go", "suggestions", "{.suggestion}", "mysuggestion", "diff", "suggestion\tfoo"}
	for _, info := range yes {
		assert.True(t, isSuggestionLanguage(info), "expected %q to be a suggestion", info)
	}
	for _, info := range no {
		assert.False(t, isSuggestionLanguage(info), "expected %q to NOT be a suggestion", info)
	}
}
