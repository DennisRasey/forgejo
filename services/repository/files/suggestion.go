// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package files

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	issues_model "forgejo.org/models/issues"
	quota_model "forgejo.org/models/quota"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup/markdown"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/typesniffer"
	"forgejo.org/modules/util"
	"forgejo.org/services/pull"
)

var (
	// ErrSuggestionNotApplicable is returned when a suggestion can't be applied: not a proposed-side code
	// comment, a pending (unsubmitted) review, or its anchored range no longer maps cleanly to the head.
	ErrSuggestionNotApplicable = util.NewInvalidArgumentErrorf("suggestion is not applicable")
	// ErrSuggestionsOverlap is returned when two suggestions in a batch target overlapping lines of one file.
	ErrSuggestionsOverlap = util.NewInvalidArgumentErrorf("suggestions overlap the same lines")
	// ErrSuggestionUnsupportedFile is returned for binary or LFS-tracked files (a line splice would corrupt them).
	ErrSuggestionUnsupportedFile = util.NewInvalidArgumentErrorf("suggestions are not supported on binary or LFS-tracked files")

	// ErrSuggestionFileTooLarge is returned when the head file exceeds the display size limit; we refuse to read
	// it fully into memory just to apply a suggestion (mirrors the file-view MaxDisplayFileSize guard).
	ErrSuggestionFileTooLarge = util.NewInvalidArgumentErrorf("file is too large to apply a suggestion")
	// ErrNoSuggestion is returned when the requested suggestion block index does not exist in the comment.
	ErrNoSuggestion = util.NewInvalidArgumentErrorf("comment has no such suggestion")
	// ErrSuggestionQuotaExceeded is returned when the head repository owner is over their storage quota.
	ErrSuggestionQuotaExceeded = errors.New("quota exceeded for the head repository owner")
)

// SuggestionEdit identifies one suggestion to apply: the ```suggestion block of a code comment.
// A code comment carries at most one suggestion
type SuggestionEdit struct {
	Comment *issues_model.Comment
}

// resolvedSuggestion is a suggestion edit after it has been resolved against the head commit.
type resolvedSuggestion struct {
	treePath  string
	startLine int // 1-based, inclusive
	endLine   int // 1-based, inclusive
	newLines  []string
	author    *user_model.User
}

// ApplySuggestions applies one or more review suggestions to the PR head branch in a single commit.
// the commit author is the suggestion's author and the committer is doer (with Co-authored-by trailers when a batch mixes several authors).
// summary/body are the user-provided commit message (summary line + optional extended description);
// an empty summary falls back to a generated default. Eligibility is re-checked here against
// the head — the persisted Invalidated flag is set asynchronously and must not be trusted for a write.
func ApplySuggestions(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, edits []*SuggestionEdit, summary, body string) (*structs.FilesResponse, error) {
	if len(edits) == 0 {
		return nil, util.NewInvalidArgumentErrorf("no suggestion to apply")
	}

	// The commit must land on the PR head branch, which for a fork lives in pr.HeadRepo.
	if err := pull.CheckHeadBranchEditable(ctx, doer, pr); err != nil {
		if errors.Is(err, pull.ErrHeadBranchNotEditable) {
			return nil, ErrSuggestionNotApplicable
		}
		return nil, err
	}

	// The commit grows the head repo, so it must respect the head-repo owner's storage quota.
	// (The QuotaTargetRepo route middleware would charge the base owner, which is wrong for a fork PR.)
	if ok, err := quota_model.EvaluateForUser(ctx, pr.HeadRepo.OwnerID, quota_model.LimitSubjectSizeGitAll); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrSuggestionQuotaExceeded
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.HeadRepo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// Resolve, read and commit all against the same head-branch tip so the SHA guard protects the right object.
	headCommitID, err := gitRepo.GetBranchCommitID(pr.HeadBranch)
	if err != nil {
		return nil, err
	}
	headCommit, err := gitRepo.GetCommit(headCommitID)
	if err != nil {
		return nil, err
	}

	byFile := map[string][]*resolvedSuggestion{}
	var authors []*user_model.User
	seenAuthor := map[string]bool{}
	for _, edit := range edits {
		rs, err := resolveSuggestionEdit(ctx, edit, pr.HeadRepo, headCommitID)
		if err != nil {
			return nil, err
		}
		byFile[rs.treePath] = append(byFile[rs.treePath], rs)
		if rs.author != nil && rs.author.Email != "" && !seenAuthor[strings.ToLower(rs.author.Email)] {
			seenAuthor[strings.ToLower(rs.author.Email)] = true
			authors = append(authors, rs.author)
		}
	}

	changeFiles := make([]*ChangeRepoFile, 0, len(byFile))
	for treePath, fileEdits := range byFile {
		// A line splice would corrupt binary or LFS-tracked files.
		filterAttribute, err := gitRepo.GitAttributeFirst(headCommitID, treePath, "filter")
		if err != nil {
			return nil, err
		}
		if filterAttribute == "lfs" {
			return nil, ErrSuggestionUnsupportedFile
		}
		blob, err := headCommit.GetBlobByPath(treePath)
		if err != nil {
			return nil, err
		}
		// Don't read an arbitrarily large file fully into memory just to splice a few lines.
		if blob.Size() >= setting.UI.MaxDisplayFileSize {
			return nil, ErrSuggestionFileTooLarge
		}
		content, err := headCommit.GetFileContent(treePath, -1)
		if err != nil {
			return nil, err
		}
		if !typesniffer.DetectContentType([]byte(content), treePath).IsRepresentableAsText() {
			return nil, ErrSuggestionUnsupportedFile
		}

		// Apply bottom-up so an earlier splice doesn't shift the line numbers of a later one; reject overlaps.
		sort.Slice(fileEdits, func(i, j int) bool { return fileEdits[i].startLine > fileEdits[j].startLine })
		for i := 1; i < len(fileEdits); i++ {
			if fileEdits[i].endLine >= fileEdits[i-1].startLine {
				return nil, ErrSuggestionsOverlap
			}
		}
		for _, fe := range fileEdits {
			content, err = spliceLines(content, fe.startLine, fe.endLine, fe.newLines)
			if err != nil {
				return nil, err
			}
		}

		changeFiles = append(changeFiles, &ChangeRepoFile{
			Operation:     "update",
			TreePath:      treePath,
			ContentReader: strings.NewReader(content),
			SHA:           blob.ID.String(),
		})
	}

	author, committer, message := suggestionCommitIdentity(doer, len(edits), authors, summary, body)
	filesResponse, err := ChangeRepoFiles(ctx, pr.HeadRepo, doer, &ChangeRepoFilesOptions{
		OldBranch: pr.HeadBranch,
		NewBranch: pr.HeadBranch,
		Message:   message,
		Files:     changeFiles,
		Author:    author,
		Committer: committer,
	})
	if err != nil {
		return nil, err
	}

	// refs/pull/N/head (what the PR diff and suggestion previews resolve against) re-syncs only
	// asynchronously after the push, but the caller reloads immediately — sync it now so the applied
	// suggestion isn't resolved against the stale head.
	if err := pull.PushToBaseRepo(ctx, pr); err != nil {
		var sha string
		if filesResponse != nil && filesResponse.Commit != nil {
			sha = filesResponse.Commit.SHA
		}
		log.Error("ApplySuggestions: sync pull ref for PR %d (head %s @ %s): %v", pr.ID, pr.HeadBranch, sha, err)
	}

	return filesResponse, nil
}

// resolveSuggestionEdit validates one edit and resolves its range against the head.
func resolveSuggestionEdit(ctx context.Context, edit *SuggestionEdit, headRepo *repo_model.Repository, headCommitID string) (*resolvedSuggestion, error) {
	comment := edit.Comment
	// Suggestions only apply to proposed-side (Line > 0) code comments.
	if comment == nil || comment.Type != issues_model.CommentTypeCode || comment.Line <= 0 {
		return nil, ErrSuggestionNotApplicable
	}
	// A pending (unsubmitted draft) review comment must not be applicable yet.
	if err := comment.LoadReview(ctx); err != nil {
		return nil, err
	}
	if comment.Review == nil || comment.Review.Type == issues_model.ReviewTypePending {
		return nil, ErrSuggestionNotApplicable
	}
	if err := comment.LoadPoster(ctx); err != nil {
		return nil, err
	}

	suggestions := markdown.ExtractSuggestions(comment.Content)
	if len(suggestions) == 0 {
		return nil, ErrNoSuggestion
	}

	// The anchored range must still map cleanly to the head, otherwise we don't know what to overwrite.
	blame, err := comment.ResolveCurrentLine(ctx, headRepo, headCommitID)
	if err != nil {
		log.Warn("ApplySuggestions: resolve line for comment %d at %s: %v", comment.ID, headCommitID, err)
		return nil, ErrSuggestionNotApplicable
	}
	if blame == nil || blame.CommitID != headCommitID {
		return nil, ErrSuggestionNotApplicable
	}
	if comment.ExtraLinesCount > 0 {
		valid, err := comment.CheckLineRangeValid(ctx, headRepo, headCommitID)
		if err != nil {
			log.Warn("ApplySuggestions: validate line range for comment %d at %s: %v", comment.ID, headCommitID, err)
			return nil, ErrSuggestionNotApplicable
		}
		if !valid {
			return nil, ErrSuggestionNotApplicable
		}
	}

	start := int(blame.LineNumber)
	return &resolvedSuggestion{
		treePath:  blame.FilePath, // head-side path (rename-aware)
		startLine: start,
		endLine:   start + int(comment.ExtraLinesCount),
		newLines:  suggestionLines(suggestions[0]),
		author:    comment.Poster,
	}, nil
}

// suggestionLines turns the raw suggestion text (LF, with at most one synthetic trailing newline added by the
// markdown parser) into the list of replacement lines (no terminators). An empty suggestion deletes the range.
func suggestionLines(suggestion string) []string {
	if suggestion == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(suggestion, "\n"), "\n")
}

// spliceLines replaces lines [start,end] (1-based, inclusive) of content with replacement, preserving the
// file's dominant line ending and its final-newline state.
func spliceLines(content string, start, end int, replacement []string) (string, error) {
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	hadFinalNewline := strings.HasSuffix(content, "\n")

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if hadFinalNewline && len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if start < 1 || end < start || end > len(lines) {
		return "", fmt.Errorf("suggestion range [%d,%d] is out of bounds (%d lines)", start, end, len(lines))
	}

	out := make([]string, 0, len(lines)-(end-start+1)+len(replacement))
	out = append(out, lines[:start-1]...)
	out = append(out, replacement...)
	out = append(out, lines[end:]...)

	result := strings.Join(out, eol)
	if hadFinalNewline && len(out) > 0 {
		result += eol
	}
	return result, nil
}

// suggestionCommitIdentity builds the commit author, committer (always doer) and message. Author is the lone
// suggestion author, else doer with Co-authored-by trailers. Message = summary (blank → default) + body + trailers
func suggestionCommitIdentity(doer *user_model.User, count int, authors []*user_model.User, summary, body string) (author, committer *IdentityOptions, message string) {
	committer = &IdentityOptions{Name: doer.Name, Email: doer.Email}
	if len(authors) == 1 {
		author = &IdentityOptions{Name: authors[0].Name, Email: authors[0].Email}
	}

	message = strings.TrimSpace(summary)
	if message == "" {
		message = "Apply suggestion"
		if count > 1 {
			message = fmt.Sprintf("Apply %d suggestions", count)
		}
	}
	if body = strings.TrimSpace(body); body != "" {
		message += "\n\n" + body
	}
	if len(authors) > 1 {
		var trailers []string
		for _, a := range authors {
			if !strings.EqualFold(a.Email, doer.Email) {
				trailers = append(trailers, fmt.Sprintf("Co-authored-by: %s <%s>", a.Name, a.Email))
			}
		}
		if len(trailers) > 0 {
			message += "\n\n" + strings.Join(trailers, "\n")
		}
	}
	return author, committer, message
}
