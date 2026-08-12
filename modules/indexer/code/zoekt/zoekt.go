// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package zoekt

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"

	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/analyze"
	"forgejo.org/modules/charset"
	"forgejo.org/modules/git"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/indexer/code/internal"
	indexer_internal "forgejo.org/modules/indexer/internal"
	inner_zoekt "forgejo.org/modules/indexer/internal/zoekt"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/typesniffer"

	"github.com/go-enry/go-enry/v2"
	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/index"
	"github.com/sourcegraph/zoekt/query"
)

var errSkipIndexing = errors.New("skip indexing")

const repoIndexerLatestVersion = 1

type zoektFormatter struct{}

type Indexer struct {
	indexer_internal.Indexer // do not composite inner_zoekt.Indexer directly to avoid exposing too much
	inner                    *inner_zoekt.Indexer
	indexDir                 string
}

func NewIndexer(indexDir string) *Indexer {
	idxer := inner_zoekt.NewIndexer(indexDir, repoIndexerLatestVersion)
	return &Indexer{
		Indexer:  idxer,
		inner:    idxer,
		indexDir: indexDir,
	}
}

func newZoektIndexBuilder(indexDir string, repo *repo_model.Repository, targetSHA string) (*index.Builder, error) {
	opts := index.Options{
		IndexDir: indexDir,
		SizeMax:  int(setting.Indexer.MaxIndexerFileSize),
		ShardMax: 512 * 1024 * 1024, // 512 MB max shard
		IsDelta:  true,
		RepositoryDescription: zoekt.Repository{
			ID:   uint32(repo.ID),
			Name: strconv.FormatInt(repo.ID, 10),
			Branches: []zoekt.RepositoryBranch{
				{
					Name:    "HEAD",
					Version: targetSHA,
				},
			},
		},
	}

	if opts.IncrementalSkipIndexing() {
		return nil, errSkipIndexing
	}

	opts.SetDefaults()

	builder, err := index.NewBuilder(opts)
	if err != nil {
		return nil, fmt.Errorf("index.newZoektIndexBuilder: %w", err)
	}

	return builder, nil
}

func (b *Indexer) addDelete(builder *index.Builder, filename string) {
	builder.MarkFileAsChangedOrRemoved(filename)
}

func (b *Indexer) addUpdate(ctx context.Context, builder *index.Builder, batchWriter io.Writer, batchReader *bufio.Reader, update internal.FileUpdate, repo *repo_model.Repository) error {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil
	}

	size := update.Size
	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = git.NewCommand(ctx, "cat-file", "-s").AddDynamicArguments(update.BlobSha).RunStdString(&git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			return err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}
	if size > setting.Indexer.MaxIndexerFileSize {
		b.addDelete(builder, update.Filename)
		return nil
	}

	if _, err := batchWriter.Write([]byte(update.BlobSha + "\n")); err != nil {
		return err
	}

	_, _, size, err = git.ReadBatchLine(batchReader)
	if err != nil {
		return err
	}

	fileContents, err := io.ReadAll(io.LimitReader(batchReader, size))
	if err != nil {
		return err
	} else if !typesniffer.DetectContentType(fileContents, update.Filename).IsText() {
		// FIXME: UTF-16 files will probably fail here
		return nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return err
	}

	builder.MarkFileAsChangedOrRemoved(update.Filename)

	branches := []string{"HEAD"}

	err = builder.Add(
		index.Document{
			Name:     update.Filename,
			Content:  charset.ToUTF8DropErrors(fileContents, charset.ConvertOpts{}),
			Branches: branches,
			Language: detectLanguage(update.Filename, fileContents),
		})
	if err != nil {
		return fmt.Errorf("error adding document with name %s: %w", update.Filename, err)
	}

	return nil
}

func detectLanguage(filename string, content []byte) string {
	lang := enry.GetLanguage(filename, content)
	return normalizeLanguage(lang)
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	builder, err := newZoektIndexBuilder(b.indexDir, repo, sha)
	if errors.Is(err, errSkipIndexing) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("error creating builder: %w", err)
	}

	if len(changes.Updates) > 0 {
		r, err := gitrepo.OpenRepository(ctx, repo)
		if err != nil {
			return err
		}
		defer r.Close()

		if err := r.WithCatFileBatch(ctx, func(w io.Writer, r *bufio.Reader) error {
			for _, update := range changes.Updates {
				err := b.addUpdate(ctx, builder, w, r, update, repo)
				if err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return nil
		}
	}

	for _, filename := range changes.RemovedFilenames {
		b.addDelete(builder, filename)
	}

	return builder.Finish()
}

func (b *Indexer) Delete(ctx context.Context, repoID int64) error {
	prefix := strconv.FormatInt(repoID, 10) + "_v"

	dir, err := os.Open(b.indexDir)
	if err != nil {
		return fmt.Errorf("open index dir: %w", err)
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	prefixLen := len(prefix)

	for _, name := range names {
		if len(name) < prefixLen {
			continue
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.HasSuffix(name, ".tmp") || strings.Contains(name, ".zoekt") {
			_ = os.Remove(filepath.Join(b.indexDir, name))
		}
	}

	return nil
}

func TransToZoektContentQueryString(s string) string {
	return fmt.Sprintf("content:\"%s\"", s)
}

// generateZoektQuery creates a Zoekt query object based on search options
func (b *Indexer) generateZoektQuery(opts *internal.SearchOptions) (query.Q, error) {
	keyword := opts.Keyword
	var contentQuery query.Q
	var err error

	// Zoekt does not support true fuzzy search.
	// CodeSearchModeFuzzy is therefore treated as a union (OR) search
	// to preserve previous behavior.
	switch opts.Mode {
	case internal.CodeSearchModeUnion, internal.CodeSearchModeFuzzy:
		fields := strings.Fields(keyword)
		if len(fields) == 0 {
			return nil, errors.New("empty keyword")
		}
		contentQuery, err = query.Parse(
			TransToZoektContentQueryString(regexp.QuoteMeta(fields[0])),
		)
		if err != nil {
			return nil, err
		}
		for _, f := range fields[1:] {
			q, err := query.Parse(
				TransToZoektContentQueryString(regexp.QuoteMeta(f)),
			)
			if err != nil {
				return nil, err
			}
			contentQuery = query.NewOr(contentQuery, q)
		}
	default:
		// Exact match
		contentQuery, err = query.Parse(
			TransToZoektContentQueryString(regexp.QuoteMeta(keyword)),
		)
		if err != nil {
			return nil, err
		}
	}

	finalQuery := contentQuery

	if len(opts.RepoIDs) > 0 {
		repoIDs := make([]uint32, len(opts.RepoIDs))
		for i, id := range opts.RepoIDs {
			repoIDs[i] = uint32(id)
		}
		finalQuery = query.NewAnd(finalQuery, query.NewRepoIDs(repoIDs...))
	}

	if opts.Filename != "" {
		prefix := strings.TrimPrefix(opts.Filename, "/")

		re, err := syntax.Parse(
			"^"+regexp.QuoteMeta(prefix),
			syntax.Perl,
		)
		if err != nil {
			return nil, err
		}

		fileQuery := &query.Regexp{
			Regexp:   re,
			FileName: true,
			Content:  false,
		}

		finalQuery = query.NewAnd(finalQuery, fileQuery)
	}

	if opts.Language != "" {
		lang := opts.Language
		if lang == "Plain Text" {
			lang = ""
		}
		finalQuery = query.NewAnd(finalQuery, &query.Language{Language: lang})
	}

	return finalQuery, nil
}

// paginateResults returns a slice of results starting from `skip` index up to `take` number of items.
func paginateResults[T any](results []T, skip, take int) []T {
	if skip >= len(results) {
		return nil
	}
	end := min(skip+take, len(results))
	return results[skip:end]
}

func getSearchResultLanguages(searchResult *zoekt.SearchResult) []*internal.SearchResultLanguages {
	languages := make(map[string]int)

	for _, file := range searchResult.Files {
		lang := normalizeLanguage(file.Language)
		languages[lang]++
	}

	searchResultLanguages := make([]*internal.SearchResultLanguages, 0, len(languages))

	for lang, count := range languages {
		searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
			Language: lang,
			Count:    count,
			Color:    enry.GetColor(lang),
		})
	}

	slices.SortFunc(searchResultLanguages, func(a, b *internal.SearchResultLanguages) int {
		if a.Count != b.Count {
			return cmp.Compare(b.Count, a.Count)
		}
		return cmp.Compare(a.Language, b.Language)
	})

	return searchResultLanguages
}

func convertZoektResult(files []zoekt.FileMatch) []*internal.SearchResult {
	results := make([]*internal.SearchResult, 0, len(files))

	for _, f := range files {
		content := string(f.Content)
		lines := strings.Split(content, "\n")

		var (
			contentLines []string
			lineNumbers  []int
			lineOffsets  []int
			matches      []internal.Match
		)

		offset := 0

		for lineIdx, line := range lines {
			lineNum := lineIdx + 1

			contentLines = append(contentLines, line)
			lineNumbers = append(lineNumbers, lineNum)
			lineOffsets = append(lineOffsets, offset)

			for _, lm := range f.LineMatches {
				if lm.LineNumber != lineNum {
					continue
				}
				for _, frag := range lm.LineFragments {
					start := int(frag.Offset)
					end := start + frag.MatchLength

					matches = append(matches, internal.Match{
						Start:      start,
						End:        end,
						LineNumber: lineNum,
					})
				}
			}

			offset += len(line) + 1
		}

		if len(matches) == 0 {
			continue
		}

		lang := normalizeLanguage(f.Language)

		results = append(results, &internal.SearchResult{
			RepoID:      int64(f.RepositoryID),
			Filename:    f.FileName,
			CommitID:    f.Version,
			Content:     strings.Join(contentLines, "\n"),
			Language:    lang,
			Color:       enry.GetColor(lang),
			Matches:     matches,
			LineNumbers: lineNumbers,
			LineOffsets: lineOffsets,
		})
	}

	return results
}

func normalizeLanguage(lang string) string {
	if lang == "" {
		return "Plain Text"
	}
	return lang
}

func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	q, err := b.generateZoektQuery(opts)
	if err != nil {
		return 0, nil, nil, err
	}

	result, err := b.inner.Searcher.Search(ctx, q, &zoekt.SearchOptions{
		Whole: true,
	})
	if err != nil {
		return 0, nil, nil, err
	}

	allHits := convertZoektResult(result.Files)

	searchResultsLanguages := getSearchResultLanguages(result)

	skip, take := opts.GetSkipTake()
	pagedHits := paginateResults(allHits, skip, take)

	total := int64(len(allHits))

	return total, pagedHits, searchResultsLanguages, nil
}

func (b *Indexer) Formatter() internal.ResultFormatter {
	return &zoektFormatter{}
}

func (f *zoektFormatter) Format(r *internal.SearchResult) (*internal.Result, error) {
	// Sort matches by start position
	slices.SortFunc(r.Matches, func(a, b internal.Match) int { return a.Start - b.Start })

	// Precompute line offsets once to avoid repeated string slicing
	lineOffsets := []int{0} // starting index of the first line
	for i, c := range r.Content {
		if c == '\n' {
			lineOffsets = append(lineOffsets, i+1)
		}
	}
	lineOffsets = append(lineOffsets, len(r.Content)) // end offset for the last line

	// Line numbers are 1-based, last line is totalLines
	totalLines := len(lineOffsets) - 1

	// Collect all lines to display (+/- 1 line around each match), clamped to the file bounds
	sortedLines := make([]int, 0, len(r.Matches)*3)
	for _, m := range r.Matches {
		for i := max(m.LineNumber-1, 1); i <= min(m.LineNumber+1, totalLines); i++ {
			sortedLines = append(sortedLines, i)
		}
	}

	// Sort lines and remove duplicates
	slices.Sort(sortedLines)
	sortedLines = slices.Compact(sortedLines)

	// Group lines into blocks (break block if distance > 2 lines)
	var blocks [][]int
	var currentBlock []int
	for _, line := range sortedLines {
		if len(currentBlock) > 0 && line > currentBlock[len(currentBlock)-1]+2 {
			blocks = append(blocks, currentBlock)
			currentBlock = nil
		}
		currentBlock = append(currentBlock, line)
	}
	if len(currentBlock) > 0 {
		blocks = append(blocks, currentBlock)
	}

	var resultLines []internal.ResultLine

	// Iterate over blocks to generate ResultLines
	for _, block := range blocks {
		startLine := block[0]
		endLine := block[len(block)-1]

		// Slice block content directly from r.Content using precomputed offsets
		startOffset := lineOffsets[startLine-1]
		endOffset := lineOffsets[endLine]
		blockContent := r.Content[startOffset:endOffset]

		// Map for highlights per line within the block
		highlightByLine := make(map[int][][2]int)
		for _, match := range r.Matches {
			if match.LineNumber < startLine || match.LineNumber > endLine {
				continue
			}
			lineInBlock := match.LineNumber - startLine
			globalLineIdx := match.LineNumber - 1

			highlightStart := match.Start - lineOffsets[globalLineIdx]
			highlightEnd := match.End - lineOffsets[globalLineIdx]

			highlightByLine[lineInBlock] = append(highlightByLine[lineInBlock], [2]int{highlightStart, highlightEnd})
		}

		// Merge overlapping highlight ranges
		var highlightRanges [][3]int
		for lineIdx, ranges := range highlightByLine {
			if len(ranges) == 0 {
				continue
			}
			slices.SortFunc(ranges, func(a, b [2]int) int { return a[0] - b[0] })
			merged := make([][2]int, 0, len(ranges))
			current := ranges[0]
			for _, r := range ranges[1:] {
				if r[0] <= current[1] {
					if r[1] > current[1] {
						current[1] = r[1]
					}
				} else {
					merged = append(merged, current)
					current = r
				}
			}
			merged = append(merged, current)

			for _, r := range merged {
				highlightRanges = append(highlightRanges, [3]int{lineIdx, r[0], r[1]})
			}
		}

		// Generate the formatted lines with highlighting
		resultLines = append(resultLines, internal.HighlightSearchResultCode(
			r.Filename,
			block,
			highlightRanges,
			blockContent,
		)...)
	}

	// Return the final search result
	return &internal.Result{
		RepoID:      r.RepoID,
		Filename:    r.Filename,
		CommitID:    r.CommitID,
		UpdatedUnix: r.UpdatedUnix,
		Language:    r.Language,
		Color:       r.Color,
		Lines:       resultLines,
	}, nil
}
