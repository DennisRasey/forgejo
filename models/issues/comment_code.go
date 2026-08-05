// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup"
	"forgejo.org/modules/markup/markdown"

	"xorm.io/builder"
)

// CodeConversation contains the comment of a given review
type CodeConversation []*Comment

// CodeConversationsAtLine contains the conversations for a given line
type CodeConversationsAtLine map[int64][]CodeConversation

// CodeConversationsAtLineAndTreePath contains the conversations for a given TreePath and line
type CodeConversationsAtLineAndTreePath map[string]CodeConversationsAtLine

func newCodeConversationsAtLineAndTreePath(ctx context.Context, comments []*Comment, repo *repo_model.Repository, headCommitID string) (CodeConversationsAtLineAndTreePath, error) {
	tree := make(CodeConversationsAtLineAndTreePath)
	for _, comment := range comments {
		blame, err := comment.ResolveCurrentLine(ctx, repo, headCommitID)
		if err != nil {
			// ResolveCurrentLine can fail in at least one known situation -- where a comment is left on a line in a
			// file that is being deleted. The blame would be for the commit that deleted the file, and a reverse git
			// blame won't work because the file is missing in the target sha.
			log.Warn("ResolveCurrentLine failed: %s", err.Error())
			// handle gracefully -- insertComment will use the original values which may be usable
			blame = nil
		} else if blame.CommitID != headCommitID {
			// Commit was made on a line that can't be reverse-blamed to the currently viewing head. This can happen
			// because:
			//  - line of code was removed between the commit it was tagged on, and the head commit
			//  - force push on the repo caused there to be no git relationship between blame.CommitID->headCommitID
			// We won't insert this comment into the comment tree because we don't know where to place it; it may appear
			// when the user views a different commit in the PR, and it will always appear on the "Conversations" tab.
			continue
		}
		tree.insertComment(comment, blame)
	}
	return tree, nil
}

func (tree CodeConversationsAtLineAndTreePath) insertComment(comment *Comment, blame *git.ReverseLineBlame) {
	treePath := comment.TreePath
	line := comment.Line
	if blame != nil {
		treePath = blame.FilePath
		line = int64(blame.LineNumber)
		if comment.Line < 0 {
			line *= -1
		}
	}

	// attempt to append comment to existing conversations (i.e. list of comments belonging to the same review)
	for i, conversation := range tree[treePath][line] {
		if conversation[0].ReviewID == comment.ReviewID {
			tree[treePath][line][i] = append(conversation, comment)
			return
		}
	}

	// no previous conversation was found at this line, create it
	if tree[treePath] == nil {
		tree[treePath] = make(map[int64][]CodeConversation)
	}

	tree[treePath][line] = append(tree[treePath][line], CodeConversation{comment})
}

// FetchCodeConversations will return a 2d-map: ["Path"]["Line"] = List of CodeConversation (one per review) for this
// line. headCommitID will be used to reverse-blame the comment into the correct path & line for the current context
// that is being viewed.
func FetchCodeConversations(ctx context.Context, issue *Issue, doer *user_model.User, showOutdatedComments bool, headCommitID string) (CodeConversationsAtLineAndTreePath, error) {
	opts := FindCommentsOptions{
		Type:    CommentTypeCode,
		IssueID: issue.ID,
	}
	comments, err := findCodeComments(ctx, opts, issue, doer, nil, showOutdatedComments)
	if err != nil {
		return nil, err
	}

	return newCodeConversationsAtLineAndTreePath(ctx, comments, issue.Repo, headCommitID)
}

// CodeComments represents comments on code by using this structure: FILENAME -> LINE (+ == proposed; - == previous) -> COMMENTS
type CodeComments map[string]map[int64][]*Comment

func fetchCodeCommentsByReview(ctx context.Context, issue *Issue, doer *user_model.User, review *Review, showOutdatedComments bool) (CodeComments, error) {
	pathToLineToComment := make(CodeComments)
	if review == nil {
		review = &Review{ID: 0}
	}
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  issue.ID,
		ReviewID: review.ID,
	}

	comments, err := findCodeComments(ctx, opts, issue, doer, review, showOutdatedComments)
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		if pathToLineToComment[comment.TreePath] == nil {
			pathToLineToComment[comment.TreePath] = make(map[int64][]*Comment)
		}
		pathToLineToComment[comment.TreePath][comment.Line] = append(pathToLineToComment[comment.TreePath][comment.Line], comment)
	}
	return pathToLineToComment, nil
}

func findCodeComments(ctx context.Context, opts FindCommentsOptions, issue *Issue, doer *user_model.User, review *Review, showOutdatedComments bool) (CommentList, error) {
	var comments CommentList
	if review == nil {
		review = &Review{ID: 0}
	}
	conds := opts.ToConds()

	if !showOutdatedComments && review.ID == 0 {
		conds = conds.And(builder.Eq{"invalidated": false})
	}

	e := db.GetEngine(ctx)
	if err := e.Where(conds).
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}

	if err := comments.LoadAttachments(ctx); err != nil {
		return nil, err
	}

	// Find all reviews by ReviewID
	reviews := make(map[int64]*Review)
	ids := make([]int64, 0, len(comments))
	for _, comment := range comments {
		if comment.ReviewID != 0 {
			ids = append(ids, comment.ReviewID)
		}
	}
	if err := e.In("id", ids).Find(&reviews); err != nil {
		return nil, err
	}

	readyComments := make(CommentList, 0, len(comments))
	for _, comment := range comments {
		if re, ok := reviews[comment.ReviewID]; ok && re != nil {
			// If the review is pending only the author can see the comments (except if the review is set)
			if review.ID == 0 && re.Type == ReviewTypePending &&
				(doer == nil || doer.ID != re.ReviewerID) {
				continue
			}
			comment.Review = re
		}
		readyComments = append(readyComments, comment)
	}

	if err := readyComments.LoadResolveDoers(ctx); err != nil {
		return nil, err
	}

	if err := readyComments.LoadReactions(ctx, issue.Repo); err != nil {
		return nil, err
	}

	for _, comment := range readyComments {
		var err error
		if comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
			Ctx: ctx,
			Links: markup.Links{
				Base: issue.Repo.Link(),
			},
			Metas: issue.Repo.ComposeMetas(ctx),
		}, comment.Content); err != nil {
			return nil, err
		}
	}

	return readyComments, nil
}

// FetchCodeConversation fetches the code conversation of a given comment (same review, treePath and line number)
func FetchCodeConversation(ctx context.Context, comment *Comment, doer *user_model.User) (CommentList, error) {
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  comment.IssueID,
		ReviewID: comment.ReviewID,
		TreePath: comment.TreePath,
		Line:     comment.Line,
	}
	return findCodeComments(ctx, opts, comment.Issue, doer, nil, true)
}
