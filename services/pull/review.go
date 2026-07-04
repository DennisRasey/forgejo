// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"
	"io"

	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/log"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
	notify_service "forgejo.org/services/notify"
)

// ErrDismissRequestOnClosedPR represents an error when an user tries to dismiss a review associated to a closed or merged PR.
type ErrDismissRequestOnClosedPR struct{}

// IsErrDismissRequestOnClosedPR checks if an error is an ErrDismissRequestOnClosedPR.
func IsErrDismissRequestOnClosedPR(err error) bool {
	_, ok := err.(ErrDismissRequestOnClosedPR)
	return ok
}

func (err ErrDismissRequestOnClosedPR) Error() string {
	return "can't dismiss a review associated to a closed or merged PR"
}

func (err ErrDismissRequestOnClosedPR) Unwrap() error {
	return util.ErrPermissionDenied
}

// checkInvalidation checks if the line of code comment got changed by another commit.
// If the line got changed the comment is going to be invalidated.
func checkInvalidation(ctx context.Context, c *issues_model.Comment, repo *repo_model.Repository, newCommitID string) error {
	reverseBlame, err := c.ResolveCurrentLine(ctx, repo, newCommitID)
	if err != nil {
		log.Warn("ResolveCurrentLine failed: %s", err.Error())
		return nil
	}
	if reverseBlame.CommitID != newCommitID {
		c.Invalidated = true
		return issues_model.UpdateCommentInvalidate(ctx, c)
	}

	// For multi-line comments, check additional lines in the range
	if c.ExtraLinesCount > 0 {
		invalidated, err := checkMultiLineInvalidation(ctx, c, repo, newCommitID)
		if err != nil {
			log.Warn("checkMultiLineInvalidation failed: %s", err.Error())
		} else if invalidated {
			c.Invalidated = true
			return issues_model.UpdateCommentInvalidate(ctx, c)
		}
	}

	return nil
}

// checkMultiLineInvalidation checks if any additional line in a multi-line comment range
// has been changed. Returns true if the comment should be invalidated.
// Uses cached results via Comment.CheckLineRangeValid.
func checkMultiLineInvalidation(ctx context.Context, c *issues_model.Comment, repo *repo_model.Repository, newCommitID string) (bool, error) {
	valid, err := c.CheckLineRangeValid(ctx, repo, newCommitID)
	if err != nil {
		return false, err
	}
	return !valid, nil
}

// InvalidateCodeComments will lookup the prs for code comments which got invalidated by change
func InvalidateCodeComments(ctx context.Context, prs issues_model.PullRequestList, doer *user_model.User, repo *repo_model.Repository, newCommitID string) error {
	if len(prs) == 0 {
		return nil
	}
	issueIDs := prs.GetIssueIDs()

	codeComments, err := db.Find[issues_model.Comment](ctx, issues_model.FindCommentsOptions{
		ListOptions: db.ListOptionsAll,
		Type:        issues_model.CommentTypeCode,
		Invalidated: optional.Some(false),
		IssueIDs:    issueIDs,
	})
	if err != nil {
		return fmt.Errorf("find code comments: %v", err)
	}
	for _, comment := range codeComments {
		if err := checkInvalidation(ctx, comment, repo, newCommitID); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCodeCommentLineRange validates the extra_lines_count of a (multi-line) code comment: it must
// not be negative and the resulting range must not span more than setting.UI.MaxCodeCommentLines lines
// (0 = no limit). It is the single source of truth shared by the web and API creation handlers.
func ValidateCodeCommentLineRange(extraLinesCount int64) error {
	if extraLinesCount < 0 {
		return fmt.Errorf("extra_lines_count must be >= 0")
	}
	if setting.UI.MaxCodeCommentLines > 0 && extraLinesCount+1 > int64(setting.UI.MaxCodeCommentLines) {
		return fmt.Errorf("a code comment may span at most %d lines", setting.UI.MaxCodeCommentLines)
	}
	return nil
}

// CreateCodeComment creates a comment on the code line
func CreateCodeComment(ctx context.Context, doer *user_model.User, gitRepo *git.Repository,
	issue *issues_model.Issue, line, extraLinesCount int64, content, treePath string, pendingReview bool,
	replyReviewID int64, beforeCommitID, latestCommitID string, attachments []string,
) (*issues_model.Comment, error) {
	var (
		existsReview bool
		err          error
	)

	// CreateCodeComment() is used for:
	// - Single comments
	// - Comments that are part of a review
	// - Comments that reply to an existing review

	if !pendingReview && replyReviewID != 0 {
		// It's not part of a review; maybe a reply to a review comment or a single comment.
		// Check if there are reviews for that line already; if there are, this is a reply
		if existsReview, err = issues_model.ReviewExists(ctx, issue, treePath, line); err != nil {
			return nil, err
		}
	}

	// Comments that are replies don't require a review header to show up in the issue view
	if !pendingReview && existsReview {
		if err = issue.LoadRepo(ctx); err != nil {
			return nil, err
		}

		comment, err := CreateCodeCommentKnownReviewID(ctx,
			doer,
			issue.Repo,
			issue,
			content,
			treePath,
			beforeCommitID,
			latestCommitID,
			line,
			extraLinesCount,
			replyReviewID,
			attachments,
		)
		if err != nil {
			return nil, err
		}

		mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comment.Content)
		if err != nil {
			return nil, err
		}

		notify_service.CreateIssueComment(ctx, doer, issue.Repo, issue, comment, mentions)

		return comment, nil
	}

	review, err := issues_model.GetCurrentReview(ctx, doer, issue)
	if err != nil {
		if !issues_model.IsErrReviewNotExist(err) {
			return nil, err
		}

		if review, err = issues_model.CreateReview(ctx, issues_model.CreateReviewOptions{
			Type:     issues_model.ReviewTypePending,
			Reviewer: doer,
			Issue:    issue,
			Official: false,
			CommitID: latestCommitID,
		}); err != nil {
			return nil, err
		}
	}

	comment, err := CreateCodeCommentKnownReviewID(ctx,
		doer,
		issue.Repo,
		issue,
		content,
		treePath,
		beforeCommitID,
		latestCommitID,
		line,
		extraLinesCount,
		review.ID,
		attachments,
	)
	if err != nil {
		return nil, err
	}

	if !pendingReview && !existsReview {
		// Submit the review we've just created so the comment shows up in the issue view
		if _, _, err = SubmitReview(ctx, doer, gitRepo, issue, issues_model.ReviewTypeComment, "", latestCommitID, nil); err != nil {
			return nil, err
		}
	}

	// NOTICE: if it's a pending review the notifications will not be fired until user submit review.

	return comment, nil
}

// CreateCodeCommentKnownReviewID creates a plain code comment at the specified line / path
func CreateCodeCommentKnownReviewID(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, content, treePath, beforeCommitID, afterCommitID string,
	line, extraLinesCount, reviewID int64, attachments []string,
) (*issues_model.Comment, error) {
	var commitID, blamedCommitID, patch string
	blamedLine := line
	if err := issue.LoadPullRequest(ctx); err != nil {
		return nil, fmt.Errorf("LoadPullRequest: %w", err)
	}
	pr := issue.PullRequest
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, fmt.Errorf("LoadBaseRepo: %w", err)
	}
	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.BaseRepo)
	if err != nil {
		return nil, fmt.Errorf("RepositoryFromContextOrOpen: %w", err)
	}
	defer closer.Close()

	invalidated := false
	head := pr.GetGitRefName()
	if line > 0 {
		if reviewID != 0 {
			first, err := issues_model.FindComments(ctx, &issues_model.FindCommentsOptions{
				ReviewID: reviewID,
				Line:     line,
				TreePath: treePath,
				Type:     issues_model.CommentTypeCode,
				ListOptions: db.ListOptions{
					PageSize: 1,
					Page:     1,
				},
			})
			if err == nil && len(first) > 0 {
				commitID = first[0].CommitSHA
				invalidated = first[0].Invalidated
				patch = first[0].Patch
			} else if err != nil && !issues_model.IsErrCommentNotExist(err) {
				return nil, fmt.Errorf("Find first comment for %d line %d path %s. Error: %w", reviewID, line, treePath, err)
			} else {
				review, err := issues_model.GetReviewByID(ctx, reviewID)
				if err == nil && len(review.CommitID) > 0 {
					head = review.CommitID
				} else if err != nil && !issues_model.IsErrReviewNotExist(err) {
					return nil, fmt.Errorf("GetReviewByID %d. Error: %w", reviewID, err)
				}
			}
		}

		if len(commitID) == 0 {
			// FIXME validate treePath
			// Get latest commit referencing the commented line
			// No need for get commit for base branch changes
			commit, lineres, err := gitRepo.LineBlame(afterCommitID, treePath, uint64(line))
			if err == nil {
				blamedCommitID = commit.ID.String()
				blamedLine = int64(lineres)
			} else if !errors.Is(err, git.ErrBlameFileDoesNotExist) && !errors.Is(err, git.ErrBlameFileNotEnoughLines) {
				return nil, fmt.Errorf("LineBlame[%s, %s, %s, %d]: %w", pr.GetGitRefName(), gitRepo.Path, treePath, line, err)
			}
		} else {
			blamedCommitID = commitID
		}
	} else {
		// Commenting on a line that was removed. In this case, what we want to track in the comment is which line of
		// code was this, in the last commit that the line of code actually existed in. We'll use a reverse git blame to
		// identify this, from the PR base -> commit being viewed.
		blame, err := gitRepo.ReverseLineBlame(beforeCommitID, treePath, uint64(-1*line), afterCommitID)
		if err != nil {
			return nil, fmt.Errorf("ReverseLineBlame[%s, %s, %d, %s]: %w", beforeCommitID, treePath, -1*line, afterCommitID, err)
		}

		// Convert to a right-hand (proposed) comment only when EVERY line of the range still exists at head
		// (whole selection unchanged); if any line was removed or modified, keep it on the previous side.
		rangeStillExists := blame.CommitID == afterCommitID
		for i := int64(1); rangeStillExists && i <= extraLinesCount; i++ {
			lineBlame, err := gitRepo.ReverseLineBlame(beforeCommitID, treePath, uint64(-1*line)+uint64(i), afterCommitID)
			if err != nil {
				return nil, fmt.Errorf("ReverseLineBlame[%s, %s, %d, %s]: %w", beforeCommitID, treePath, -1*line+i, afterCommitID, err)
			}
			if lineBlame.CommitID != afterCommitID {
				rangeStillExists = false
			}
		}

		switch {
		case rangeStillExists:
			commit, lineres, err := gitRepo.LineBlame(afterCommitID, treePath, blame.LineNumber)
			if err == nil {
				blamedCommitID = commit.ID.String()
				blamedLine = int64(lineres)
			} else if !errors.Is(err, git.ErrBlameFileDoesNotExist) && !errors.Is(err, git.ErrBlameFileNotEnoughLines) {
				return nil, fmt.Errorf("LineBlame[%s, %s, %s, %d]: %w", pr.GetGitRefName(), gitRepo.Path, treePath, line, err)
			}
		case blame.CommitID == afterCommitID:
			// First line still exists but a later one changed
			blamedCommitID = beforeCommitID
			// retain negative line numbering to identify we're commenting on the "previous" side of the diff
			blamedLine = line
		default:
			blamedCommitID = blame.CommitID
			// retain negative line numbering to identify we're commenting on the "previous" side of the diff
			blamedLine = -1 * int64(blame.LineNumber)
		}
	}

	// Only fetch diff if comment is review comment
	if len(patch) == 0 && reviewID != 0 {
		if len(commitID) == 0 {
			commitID, err = gitRepo.GetRefCommitID(head)
			if err != nil {
				return nil, fmt.Errorf("GetRefCommitID[%s]: %w", head, err)
			}
		}
		if len(blamedCommitID) == 0 {
			blamedCommitID = commitID
		}
		reader, writer := io.Pipe()
		defer func() {
			_ = reader.Close()
			_ = writer.Close()
		}()
		go func() {
			if err := git.GetRepoRawDiffForFile(gitRepo, beforeCommitID, afterCommitID, git.RawDiffNormal, treePath, writer); err != nil {
				_ = writer.CloseWithError(fmt.Errorf("GetRawDiffForLine[%s, %s, %s, %s]: %w", gitRepo.Path, pr.MergeBase, afterCommitID, treePath, err))
				return
			}
			_ = writer.Close()
		}()

		// For multi-line comments, center the patch on the last line and expand context to include the full range
		displayLine := int64((&issues_model.Comment{Line: line, ExtraLinesCount: extraLinesCount}).UnsignedDisplayLine())
		contextLines := setting.UI.CodeCommentLines + int(extraLinesCount)
		patch, err = git.CutDiffAroundLine(reader, displayLine, line < 0, contextLines)
		if err != nil {
			// The anchored commit may be gone (e.g. force-push before a migration), so the hunk
			// can't be produced. Store the comment without context, like the importer.
			log.Warn("CreateCodeComment: storing comment without diff context: %v", err)
			patch = ""
		}
	}
	return issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:            issues_model.CommentTypeCode,
		Doer:            doer,
		Repo:            repo,
		Issue:           issue,
		Content:         content,
		LineNum:         blamedLine,
		ExtraLinesCount: extraLinesCount,
		TreePath:        treePath,
		CommitSHA:       blamedCommitID,
		ReviewID:        reviewID,
		Patch:           patch,
		Invalidated:     invalidated,
		Attachments:     attachments,
	})
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(ctx context.Context, doer *user_model.User, gitRepo *git.Repository, issue *issues_model.Issue, reviewType issues_model.ReviewType, content, commitID string, attachmentUUIDs []string) (*issues_model.Review, *issues_model.Comment, error) {
	if err := issue.LoadPullRequest(ctx); err != nil {
		return nil, nil, err
	}

	pr := issue.PullRequest
	var stale bool
	if reviewType != issues_model.ReviewTypeApprove && reviewType != issues_model.ReviewTypeReject {
		stale = false
	} else {
		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return nil, nil, err
		}

		if headCommitID == commitID {
			stale = false
		} else {
			testPatchCtx, err := getTestPatchCtx(ctx, pr, true)
			defer testPatchCtx.close()
			if err != nil {
				return nil, nil, err
			}

			stale, err = testPatchCtx.gitRepo.CheckIfDiffDiffers(testPatchCtx.baseRev, commitID, headCommitID, testPatchCtx.env)
			if err != nil {
				return nil, nil, fmt.Errorf("CheckIfDiffDiffers: %w", err)
			}
		}
	}

	review, comm, err := issues_model.SubmitReview(ctx, doer, issue, reviewType, content, commitID, stale, attachmentUUIDs)
	if err != nil {
		return nil, nil, err
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comm.Content)
	if err != nil {
		return nil, nil, err
	}

	notify_service.PullRequestReview(ctx, pr, review, comm, mentions)

	for _, lines := range review.CodeComments {
		for _, comments := range lines {
			for _, codeComment := range comments {
				mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, codeComment.Content)
				if err != nil {
					return nil, nil, err
				}
				notify_service.PullRequestCodeComment(ctx, pr, codeComment, mentions)
			}
		}
	}

	return review, comm, nil
}

// DismissApprovalReviews dismiss all approval reviews because of new commits
func DismissApprovalReviews(ctx context.Context, doer *user_model.User, pull *issues_model.PullRequest) error {
	reviews, err := issues_model.FindReviews(ctx, issues_model.FindReviewOptions{
		ListOptions: db.ListOptionsAll,
		IssueID:     pull.IssueID,
		Types:       []issues_model.ReviewType{issues_model.ReviewTypeApprove},
		Dismissed:   optional.Some(false),
	})
	if err != nil {
		return err
	}

	if err := reviews.LoadIssues(ctx); err != nil {
		return err
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, review := range reviews {
			if err := issues_model.DismissReview(ctx, review, true); err != nil {
				return err
			}

			comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
				Doer:     doer,
				Content:  "New commits pushed, approval review dismissed automatically according to repository settings",
				Type:     issues_model.CommentTypeDismissReview,
				ReviewID: review.ID,
				Issue:    review.Issue,
				Repo:     review.Issue.Repo,
			})
			if err != nil {
				return err
			}

			comment.Review = review
			comment.Poster = doer
			comment.Issue = review.Issue

			notify_service.PullReviewDismiss(ctx, doer, review, comment)
		}
		return nil
	})
}

// DismissReview dismissing stale review by repo admin
func DismissReview(ctx context.Context, reviewID, repoID int64, message string, doer *user_model.User, isDismiss, dismissPriors bool) (comment *issues_model.Comment, err error) {
	review, err := issues_model.GetReviewByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	if review.Type != issues_model.ReviewTypeApprove && review.Type != issues_model.ReviewTypeReject {
		return nil, errors.New("not need to dismiss this review because it's type is not Approve or change request")
	}

	// load data for notify
	if err := review.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	// Check if the review's repoID is the one we're currently expecting.
	if review.Issue.RepoID != repoID {
		return nil, errors.New("reviews's repository is not the same as the one we expect")
	}

	issue := review.Issue

	if issue.IsClosed {
		return nil, ErrDismissRequestOnClosedPR{}
	}

	if issue.IsPull {
		if err := issue.LoadPullRequest(ctx); err != nil {
			return nil, err
		}
		if issue.PullRequest.HasMerged {
			return nil, ErrDismissRequestOnClosedPR{}
		}
	}

	if err := issues_model.DismissReview(ctx, review, isDismiss); err != nil {
		return nil, err
	}

	if dismissPriors {
		reviews, err := issues_model.FindReviews(ctx, issues_model.FindReviewOptions{
			IssueID:    review.IssueID,
			ReviewerID: review.ReviewerID,
			Dismissed:  optional.Some(false),
		})
		if err != nil {
			return nil, err
		}
		for _, oldReview := range reviews {
			if err = issues_model.DismissReview(ctx, oldReview, true); err != nil {
				return nil, err
			}
		}
	}

	if !isDismiss {
		return nil, nil
	}

	if err := review.Issue.LoadAttributes(ctx); err != nil {
		return nil, err
	}

	comment, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Doer:     doer,
		Content:  message,
		Type:     issues_model.CommentTypeDismissReview,
		ReviewID: review.ID,
		Issue:    review.Issue,
		Repo:     review.Issue.Repo,
	})
	if err != nil {
		return nil, err
	}

	comment.Review = review
	comment.Poster = doer
	comment.Issue = review.Issue

	notify_service.PullReviewDismiss(ctx, doer, review, comment)

	return comment, nil
}
