// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package gitdiff

import (
	"context"
	"fmt"
	"strings"

	issues_model "forgejo.org/models/issues"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup/markdown"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/typesniffer"
)

// SuggestionDiff is the rendered before/after view of one ```suggestion block; Index is its 0-based
// position among the comment's suggestion blocks, so the frontend can pair it with the right code block.
type SuggestionDiff struct {
	Diff *Diff
}

// SuggestionDiffs renders the before/after diff of each ```suggestion block in a proposed-side code
func SuggestionDiffs(ctx context.Context, comment *issues_model.Comment) []*SuggestionDiff {
	// Only proposed-side (Line > 0) code comments can carry an applicable suggestion.
	if comment == nil || comment.Type != issues_model.CommentTypeCode || comment.Line <= 0 {
		return nil
	}

	// This runs as a template helper during page render; never let a panic (patch parsing,
	// git access, ...) become a 500 — degrade to a plain code block instead. Mirrors CommentMustAsDiff.
	defer func() {
		if r := recover(); r != nil {
			log.Error("SuggestionDiffs panic for comment %d: %v\n%s", comment.ID, r, log.Stack(2))
		}
	}()

	suggestions := markdown.ExtractSuggestions(comment.Content)
	if len(suggestions) == 0 {
		return nil
	}

	if err := comment.LoadIssue(ctx); err != nil {
		log.Warn("SuggestionDiffs: LoadIssue for comment %d: %v", comment.ID, err)
		return nil
	}
	issue := comment.Issue
	if err := issue.LoadPullRequest(ctx); err != nil || issue.PullRequest == nil {
		return nil
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Warn("SuggestionDiffs: LoadRepo for comment %d: %v", comment.ID, err)
		return nil
	}
	repo := issue.Repo

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		log.Warn("SuggestionDiffs: open repo for comment %d: %v", comment.ID, err)
		return nil
	}
	defer closer.Close()

	headCommitID, err := gitRepo.GetRefCommitID(issue.PullRequest.GetGitRefName())
	if err != nil {
		return nil
	}

	// The anchored range must still resolve cleanly to the head; otherwise we don't
	// have a trustworthy "before" to diff against, so leave the block as plain code.
	blame, err := comment.ResolveCurrentLine(ctx, repo, headCommitID)
	if err != nil || blame == nil || blame.CommitID != headCommitID {
		return nil
	}
	if comment.ExtraLinesCount > 0 {
		valid, err := comment.CheckLineRangeValid(ctx, repo, headCommitID)
		if err != nil || !valid {
			return nil
		}
	}

	startLine := blame.LineNumber
	endLine := startLine + uint64(comment.ExtraLinesCount)
	treePath := blame.FilePath // head-side path (handles renames)

	// An LFS-tracked or binary file can never carry an applicable suggestion (a line splice would
	// corrupt it), so don't render a misleading before/after preview — leave the block as plain code.
	if attr, err := gitRepo.GitAttributeFirst(headCommitID, treePath, "filter"); err != nil || attr == "lfs" {
		return nil
	}

	headCommit, err := gitRepo.GetCommit(headCommitID)
	if err != nil {
		return nil
	}

	// Retrieve the blob to check its size before trying to read the content, to avoid OOM on huge files
	blob, err := headCommit.GetBlobByPath(treePath)
	if err != nil || blob.Size() >= setting.UI.MaxDisplayFileSize {
		return nil
	}
	content, err := headCommit.GetFileContent(treePath, -1)
	if err != nil {
		return nil
	}
	if !typesniffer.DetectContentType([]byte(content), treePath).IsRepresentableAsText() {
		return nil
	}
	fileLines := splitLines(content)
	if startLine < 1 || endLine > uint64(len(fileLines)) {
		return nil
	}
	original := fileLines[startLine-1 : endLine]

	result := make([]*SuggestionDiff, 0, len(suggestions))
	for i, suggestion := range suggestions {
		patch := synthesizeSuggestionPatch(treePath, startLine, original, splitLines(suggestion))
		diff, err := ParsePatch(ctx, setting.Git.MaxGitDiffLines,
			setting.Git.MaxGitDiffLineCharacters, strings.NewReader(patch))
		if err != nil || len(diff.Files) == 0 {
			log.Warn("SuggestionDiffs: ParsePatch for comment %d block %d: %v", comment.ID, i, err)
			continue
		}
		result = append(result, &SuggestionDiff{Diff: diff})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// splitLines normalizes CRLF to LF, splits on "\n" and drops the trailing empty element produced
// by a final newline, so the result is the list of actual lines (without terminators).
// Normalizing here keeps the head "before" lines aligned with the LF-normalized suggestion text (and with the
// apply path), so a CRLF file doesn't show spurious changes in the preview.
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// synthesizeSuggestionPatch builds a minimal unified-diff patch replacing the
// original lines (at startLine) with the suggestion lines, suitable for ParsePatch.
func synthesizeSuggestionPatch(treePath string, startLine uint64, original, suggestion []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", treePath, treePath)
	fmt.Fprintf(&b, "--- a/%s\n", treePath)
	fmt.Fprintf(&b, "+++ b/%s\n", treePath)
	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", startLine, len(original), startLine, len(suggestion))
	for _, line := range original {
		b.WriteString("-")
		b.WriteString(line)
		b.WriteString("\n")
	}
	for _, line := range suggestion {
		b.WriteString("+")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
