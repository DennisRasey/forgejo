// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package zoekt

import (
	"testing"

	"forgejo.org/modules/indexer/code/internal"

	"github.com/sourcegraph/zoekt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertZoektResult_BasicMatch(t *testing.T) {
	content := "hello world\nsecond line\n"

	files := []zoekt.FileMatch{
		{
			RepositoryID: 1,
			FileName:     "main.go",
			Version:      "commit123",
			Language:     "Go",
			Content:      []byte(content),
			LineMatches: []zoekt.LineMatch{
				{
					LineNumber: 1,
					LineFragments: []zoekt.LineFragmentMatch{
						{
							Offset:      6, // "world"
							MatchLength: 5,
						},
					},
				},
			},
		},
	}

	results := convertZoektResult(files)
	require.Len(t, results, 1)

	res := results[0]

	assert.Equal(t, int64(1), res.RepoID)
	assert.Equal(t, "main.go", res.Filename)
	assert.Equal(t, "commit123", res.CommitID)
	assert.Equal(t, "Go", res.Language)
	assert.Equal(t, content, res.Content)

	require.Len(t, res.Matches, 1)
	assert.Equal(t, 6, res.Matches[0].Start)
	assert.Equal(t, 11, res.Matches[0].End)
	assert.Equal(t, 1, res.Matches[0].LineNumber)

	assert.Equal(t, []int{1, 2, 3}, res.LineNumbers)
	assert.Equal(t, []int{0, 12, 24}, res.LineOffsets)
}

func TestConvertZoektResult_PlainTextFallback(t *testing.T) {
	files := []zoekt.FileMatch{
		{
			RepositoryID: 2,
			FileName:     "README",
			Version:      "commit456",
			Content:      []byte("just text"),
			LineMatches: []zoekt.LineMatch{
				{
					LineNumber: 1,
					LineFragments: []zoekt.LineFragmentMatch{
						{Offset: 0, MatchLength: 4},
					},
				},
			},
		},
	}

	results := convertZoektResult(files)
	require.Len(t, results, 1)

	assert.Equal(t, "Plain Text", results[0].Language)
}

func TestGetSearchResultLanguages(t *testing.T) {
	searchResult := &zoekt.SearchResult{
		Files: []zoekt.FileMatch{
			{Language: "Go"},
			{Language: "Go"},
			{Language: "Rust"},
			{Language: ""},
		},
	}

	stats := getSearchResultLanguages(searchResult)

	require.Len(t, stats, 3)

	assert.Equal(t, "Go", stats[0].Language)
	assert.Equal(t, 2, stats[0].Count)

	assert.Equal(t, "Plain Text", stats[1].Language)
	assert.Equal(t, 1, stats[1].Count)

	assert.Equal(t, "Rust", stats[2].Language)
	assert.Equal(t, 1, stats[2].Count)
}

func TestGenerateZoektQuery_Union(t *testing.T) {
	indexer := &Indexer{}

	opts := &internal.SearchOptions{
		Keyword: "foo bar",
		Mode:    internal.CodeSearchModeUnion,
		RepoIDs: []int64{1, 2},
	}

	q, err := indexer.generateZoektQuery(opts)
	require.NoError(t, err)
	require.NotNil(t, q)
}

func TestGenerateZoektQuery_SpecialCharacters(t *testing.T) {
	indexer := &Indexer{}

	opts := &internal.SearchOptions{
		Keyword: `foo.bar+baz[qux]`,
		Mode:    internal.CodeSearchModeExact,
	}

	q, err := indexer.generateZoektQuery(opts)
	require.NoError(t, err)
	require.NotNil(t, q)
}

func TestZoektFormatter_Format(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		content       string
		lineNumber    int
		offset        int // offset relative to the start of the file
		matchLength   int
		expectedLines []int
		expectedText  string
	}{
		{
			name:          "single line without trailing newline",
			content:       "test",
			lineNumber:    1,
			offset:        0,
			matchLength:   4,
			expectedLines: []int{1},
			expectedText:  "test",
		},
		{
			name:          "match on the last line without trailing newline",
			content:       "one\ntwo\ntest",
			lineNumber:    3,
			offset:        8,
			matchLength:   4,
			expectedLines: []int{2, 3},
			expectedText:  "test",
		},
		{
			name:          "match on the last line with trailing newline",
			content:       "one\ntwo\ntest\n",
			lineNumber:    3,
			offset:        8,
			matchLength:   4,
			expectedLines: []int{2, 3},
			expectedText:  "test",
		},
		{
			name:          "match on the first line",
			content:       "test\ntwo\nthree",
			lineNumber:    1,
			offset:        0,
			matchLength:   4,
			expectedLines: []int{1, 2},
			expectedText:  "test",
		},
		{
			name:          "match on a line in the middle",
			content:       "one\ntest\nthree\nfour",
			lineNumber:    2,
			offset:        4,
			matchLength:   4,
			expectedLines: []int{1, 2, 3},
			expectedText:  "test",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			files := []zoekt.FileMatch{
				{
					RepositoryID: 1,
					FileName:     "example.txt",
					Version:      "commit123",
					Content:      []byte(testCase.content),
					LineMatches: []zoekt.LineMatch{
						{
							LineNumber: testCase.lineNumber,
							LineFragments: []zoekt.LineFragmentMatch{
								{
									Offset:      uint32(testCase.offset),
									MatchLength: testCase.matchLength,
								},
							},
						},
					},
				},
			}

			results := convertZoektResult(files)
			require.Len(t, results, 1)

			formatter := &zoektFormatter{}
			result, err := formatter.Format(results[0])
			require.NoError(t, err)

			lineNumbers := make([]int, 0, len(result.Lines))
			for _, line := range result.Lines {
				lineNumbers = append(lineNumbers, line.Num)
			}
			assert.Equal(t, testCase.expectedLines, lineNumbers)

			var matchedLine string
			for _, line := range result.Lines {
				if line.Num == testCase.lineNumber {
					matchedLine = string(line.FormattedContent)
				}
			}
			assert.Contains(t, matchedLine, `<span class="search-highlight">`+testCase.expectedText+`</span>`)
		})
	}
}

func TestConvertZoektResult_IgnoresFilenameMatches(t *testing.T) {
	files := []zoekt.FileMatch{
		{
			RepositoryID: 1,
			FileName:     "foo.go",
			Version:      "commit123",
			Content:      []byte("package main\n"),
			LineMatches: []zoekt.LineMatch{
				{
					FileName: true, // filename-only match
				},
			},
		},
	}

	results := convertZoektResult(files)
	assert.Empty(t, results)
}
