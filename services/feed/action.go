// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"context"
	"fmt"
	"path"
	"strings"

	activities_model "forgejo.org/models/activities"
	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/graceful"
	"forgejo.org/modules/json"
	"forgejo.org/modules/log"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/process"
	"forgejo.org/modules/queue"
	"forgejo.org/modules/repository"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"
	federation_service "forgejo.org/services/federation"
	notify_service "forgejo.org/services/notify"
)

const maxRetries = 3

type QueueableAction struct {
	ID          int64
	UserID      int64
	OpType      activities_model.ActionType
	ActUserID   int64
	RepoID      int64
	CommentID   int64
	RefName     string
	IsPrivate   bool
	Content     string
	CreatedUnix timeutil.TimeStamp
}

type actionNotifier struct {
	notify_service.NullNotifier
}

type notificationQueueItem struct {
	Action          QueueableAction
	LocalCount      uint
	LocalOut        optional.Option[[]QueueableAction]
	FederationCount uint
}

var (
	_                 notify_service.Notifier = &actionNotifier{}
	notificationQueue *queue.WorkerPoolQueue[notificationQueueItem]
)

func (a *QueueableAction) ToAction() activities_model.Action {
	return activities_model.Action{
		ID:          a.ID,
		UserID:      a.UserID,
		OpType:      a.OpType,
		ActUserID:   a.ActUserID,
		RepoID:      a.RepoID,
		CommentID:   a.CommentID,
		RefName:     a.RefName,
		IsPrivate:   a.IsPrivate,
		Content:     a.Content,
		CreatedUnix: a.CreatedUnix,
	}
}

func FromAction(a activities_model.Action) QueueableAction {
	return QueueableAction{
		ID:          a.ID,
		UserID:      a.UserID,
		OpType:      a.OpType,
		ActUserID:   a.UserID,
		RepoID:      a.RepoID,
		CommentID:   a.CommentID,
		RefName:     a.RefName,
		IsPrivate:   a.IsPrivate,
		Content:     a.Content,
		CreatedUnix: a.CreatedUnix,
	}
}

func initNotificationQueue() error {
	notificationQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "notification_queue", notificationQueueHandler)
	if notificationQueue == nil {
		return fmt.Errorf("Failed to create notification_queue")
	}

	go graceful.GetManager().RunWithCancel(notificationQueue)

	return nil
}

func notificationQueueHandler(notificationItems ...notificationQueueItem) (unhandled []notificationQueueItem) {
	for _, notification := range notificationItems {
		err := deliverNotification(&notification)
		if err != nil {
			log.Error("Failed to deliver notification: %e", err)

			if notification.LocalCount > maxRetries || notification.FederationCount > maxRetries {
				log.Warn("Dropping notification after exceeding retry count (%d)", maxRetries)
				continue
			}

			unhandled = append(unhandled, notification)
		}
	}

	return unhandled
}

func deliverNotification(notification *notificationQueueItem) error {
	hammerCtx := graceful.GetManager().HammerContext()
	ctx, _, finished := process.GetManager().AddContext(hammerCtx, "Delivering notification")
	defer finished()

	if !notification.LocalOut.Has() {
		action := notification.Action.ToAction()
		out, err := activities_model.NotifyWatchers(ctx, &action)
		if err != nil {
			notification.LocalCount++
			return err
		}

		// convert database Actions to QueueableActions
		queueableActionOut := []QueueableAction{}
		for _, a := range out {
			queueableActionOut = append(queueableActionOut, FromAction(a))
		}

		notification.LocalOut = optional.Some(queueableActionOut)
	}

	_, out := notification.LocalOut.Get()
	// convert QueueableActions to database Actions
	actionOut := []activities_model.Action{}
	for _, a := range out {
		actionOut = append(actionOut, a.ToAction())
	}

	err := federation_service.NotifyActivityPubFollowers(ctx, actionOut)
	if err != nil {
		notification.FederationCount++
		return err
	}

	return nil
}

func Init() error {
	notify_service.RegisterNotifier(NewNotifier())
	return initNotificationQueue()
}

// NewNotifier create a new actionNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &actionNotifier{}
}

func notifyAll(ctx context.Context, action QueueableAction) {
	db.AfterTx(ctx, func() {
		err := notificationQueue.Push(notificationQueueItem{
			Action:          action,
			LocalCount:      0,
			LocalOut:        optional.None[[]QueueableAction](),
			FederationCount: 0,
		})
		if err != nil {
			log.Error("Failed to enqueue notification for delivery: %s", err.Error())
		}
	})
}

func notifyAllActions(ctx context.Context, acts []*activities_model.Action) error {
	out, err := activities_model.NotifyWatchersActions(ctx, acts)
	if err != nil {
		return err
	}
	return federation_service.NotifyActivityPubFollowers(ctx, out)
}

func (a *actionNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	repo := issue.Repo

	notifyAll(ctx, QueueableAction{
		ActUserID: issue.Poster.ID,
		OpType:    activities_model.ActionCreateIssue,
		Content:   encodeContent(fmt.Sprintf("%d", issue.Index), issue.Title),
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
	})
}

// IssueChangeStatus notifies close or reopen issue to notifiers
func (a *actionNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, prInfo *issues_model.PRNotificationInfo, issue *issues_model.Issue, actionComment *issues_model.Comment, closeOrReopen bool) {
	// Compose comment action, could be plain comment, close or reopen issue/pull request.
	// This object will be used to notify watchers in the end of function.
	act := QueueableAction{
		ActUserID: doer.ID,
		Content:   encodeContent(fmt.Sprintf("%d", issue.Index), ""),
		RepoID:    issue.Repo.ID,
		CommentID: actionComment.ID,
		IsPrivate: issue.Repo.IsPrivate,
	}
	// Check comment type.
	if closeOrReopen {
		act.OpType = activities_model.ActionCloseIssue
		if issue.IsPull {
			act.OpType = activities_model.ActionClosePullRequest
		}
	} else {
		act.OpType = activities_model.ActionReopenIssue
		if issue.IsPull {
			act.OpType = activities_model.ActionReopenPullRequest
		}
	}

	// Notify watchers for whatever action comes in, ignore if no action type.
	notifyAll(ctx, act)
}

// CreateIssueComment notifies comment on an issue to notifiers
func (a *actionNotifier) CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository,
	issue *issues_model.Issue, comment *issues_model.Comment, mentions []*user_model.User,
) {
	act := QueueableAction{
		ActUserID: doer.ID,
		RepoID:    issue.Repo.ID,
		CommentID: comment.ID,
		IsPrivate: issue.Repo.IsPrivate,
		Content:   encodeContent(fmt.Sprintf("%d", issue.Index), abbreviatedComment(comment.Content)),
	}

	if issue.IsPull {
		act.OpType = activities_model.ActionCommentPull
	} else {
		act.OpType = activities_model.ActionCommentIssue
	}

	// Notify watchers for whatever action comes in, ignore if no action type.
	notifyAll(ctx, act)
}

func (a *actionNotifier) NewPullRequest(ctx context.Context, pull *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pull.LoadIssue(ctx); err != nil {
		log.Error("pull.LoadIssue: %v", err)
		return
	}
	if err := pull.Issue.LoadRepo(ctx); err != nil {
		log.Error("pull.Issue.LoadRepo: %v", err)
		return
	}
	if err := pull.Issue.LoadPoster(ctx); err != nil {
		log.Error("pull.Issue.LoadPoster: %v", err)
		return
	}

	notifyAll(ctx, QueueableAction{
		ActUserID: pull.Issue.Poster.ID,
		OpType:    activities_model.ActionCreatePullRequest,
		Content:   encodeContent(fmt.Sprintf("%d", pull.Issue.Index), pull.Issue.Title),
		RepoID:    pull.Issue.Repo.ID,
		IsPrivate: pull.Issue.Repo.IsPrivate,
	})
}

func (a *actionNotifier) RenameRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldRepoName string) {
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    activities_model.ActionRenameRepo,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	})
}

func (a *actionNotifier) TransferRepository(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, oldOwnerName string) {
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    activities_model.ActionTransferRepo,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
		Content:   path.Join(oldOwnerName, repo.Name),
	})
}

func (a *actionNotifier) CreateRepository(ctx context.Context, doer, u *user_model.User, repo *repo_model.Repository) {
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    activities_model.ActionCreateRepo,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
	})
}

func (a *actionNotifier) ForkRepository(ctx context.Context, doer *user_model.User, oldRepo, repo *repo_model.Repository) {
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    activities_model.ActionCreateRepo,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
	})
}

func (a *actionNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := review.LoadReviewer(ctx); err != nil {
		log.Error("LoadReviewer '%d/%d': %v", review.ID, review.ReviewerID, err)
		return
	}
	if err := review.LoadCodeComments(ctx); err != nil {
		log.Error("LoadCodeComments '%d/%d': %v", review.Reviewer.ID, review.ID, err)
		return
	}

	actions := make([]*activities_model.Action, 0, 10)
	for _, lines := range review.CodeComments {
		for _, comments := range lines {
			for _, comm := range comments {
				actions = append(actions, &activities_model.Action{
					ActUserID: review.Reviewer.ID,
					ActUser:   review.Reviewer,
					Content:   encodeContent(fmt.Sprintf("%d", review.Issue.Index), abbreviatedComment(comm.Content)),
					OpType:    activities_model.ActionCommentPull,
					RepoID:    review.Issue.RepoID,
					Repo:      review.Issue.Repo,
					IsPrivate: review.Issue.Repo.IsPrivate,
					Comment:   comm,
					CommentID: comm.ID,
				})
			}
		}
	}

	if review.Type != issues_model.ReviewTypeComment || strings.TrimSpace(comment.Content) != "" {
		action := &activities_model.Action{
			ActUserID: review.Reviewer.ID,
			ActUser:   review.Reviewer,
			Content:   encodeContent(fmt.Sprintf("%d", review.Issue.Index), abbreviatedComment(comment.Content)),
			RepoID:    review.Issue.RepoID,
			Repo:      review.Issue.Repo,
			IsPrivate: review.Issue.Repo.IsPrivate,
			Comment:   comment,
			CommentID: comment.ID,
		}

		switch review.Type {
		case issues_model.ReviewTypeApprove:
			action.OpType = activities_model.ActionApprovePullRequest
		case issues_model.ReviewTypeReject:
			action.OpType = activities_model.ActionRejectPullRequest
		default:
			action.OpType = activities_model.ActionCommentPull
		}

		actions = append(actions, action)
	}

	if err := notifyAllActions(ctx, actions); err != nil {
		log.Error("notify watchers '%d/%d': %v", review.Reviewer.ID, review.Issue.RepoID, err)
	}
}

func (*actionNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    activities_model.ActionMergePullRequest,
		Content:   encodeContent(fmt.Sprintf("%d", pr.Issue.Index), pr.Issue.Title),
		RepoID:    pr.Issue.Repo.ID,
		IsPrivate: pr.Issue.Repo.IsPrivate,
	})
}

func (*actionNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    activities_model.ActionAutoMergePullRequest,
		Content:   encodeContent(fmt.Sprintf("%d", pr.Issue.Index), pr.Issue.Title),
		RepoID:    pr.Issue.Repo.ID,
		IsPrivate: pr.Issue.Repo.IsPrivate,
	})
}

func (*actionNotifier) PullReviewDismiss(ctx context.Context, doer *user_model.User, review *issues_model.Review, comment *issues_model.Comment) {
	if err := review.LoadReviewer(ctx); err != nil {
		log.Error("LoadReviewer '%d/%d': %v", review.ID, review.ReviewerID, err)
		return
	}
	reviewerName := review.Reviewer.Name
	if len(review.OriginalAuthor) > 0 {
		reviewerName = review.OriginalAuthor
	}
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    activities_model.ActionPullReviewDismissed,
		Content:   encodeContent(fmt.Sprintf("%d", review.Issue.Index), reviewerName, abbreviatedComment(comment.Content)),
		RepoID:    review.Issue.Repo.ID,
		IsPrivate: review.Issue.Repo.IsPrivate,
		CommentID: comment.ID,
	})
}

func (a *actionNotifier) PushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	commits = prepareCommitsForFeed(commits)

	data, err := json.Marshal(commits)
	if err != nil {
		log.Error("Marshal: %v", err)
		return
	}

	opType := activities_model.ActionCommitRepo

	// Check it's tag push or branch.
	if opts.RefFullName.IsTag() {
		opType = activities_model.ActionPushTag
		if opts.IsDelRef() {
			opType = activities_model.ActionDeleteTag
		}
	} else if opts.IsDelRef() {
		opType = activities_model.ActionDeleteBranch
	}

	notifyAll(ctx, QueueableAction{
		ActUserID: pusher.ID,
		OpType:    opType,
		Content:   string(data),
		RepoID:    repo.ID,
		RefName:   opts.RefFullName.String(),
		IsPrivate: repo.IsPrivate,
	})
}

func (a *actionNotifier) CreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	opType := activities_model.ActionCommitRepo
	if refFullName.IsTag() {
		// has sent same action in `PushCommits`, so skip it.
		return
	}
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    opType,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	})
}

func (a *actionNotifier) DeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	opType := activities_model.ActionDeleteBranch
	if refFullName.IsTag() {
		// has sent same action in `PushCommits`, so skip it.
		return
	}
	notifyAll(ctx, QueueableAction{
		ActUserID: doer.ID,
		OpType:    opType,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	})
}

func (a *actionNotifier) SyncPushCommits(ctx context.Context, pusher *user_model.User, repo *repo_model.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	commits = prepareCommitsForFeed(commits)

	data, err := json.Marshal(commits)
	if err != nil {
		log.Error("json.Marshal: %v", err)
		return
	}

	notifyAll(ctx, QueueableAction{
		ActUserID: repo.OwnerID,
		OpType:    activities_model.ActionMirrorSyncPush,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
		RefName:   opts.RefFullName.String(),
		Content:   string(data),
	})
}

func (a *actionNotifier) SyncCreateRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName, refID string) {
	notifyAll(ctx, QueueableAction{
		ActUserID: repo.OwnerID,
		OpType:    activities_model.ActionMirrorSyncCreate,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	})
}

func (a *actionNotifier) SyncDeleteRef(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, refFullName git.RefName) {
	notifyAll(ctx, QueueableAction{
		ActUserID: repo.OwnerID,
		OpType:    activities_model.ActionMirrorSyncDelete,
		RepoID:    repo.ID,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName.String(),
	})
}

func (a *actionNotifier) NewRelease(ctx context.Context, rel *repo_model.Release) {
	if err := rel.LoadAttributes(ctx); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}
	notifyAll(ctx, QueueableAction{
		ActUserID: rel.PublisherID,
		OpType:    activities_model.ActionPublishRelease,
		RepoID:    rel.RepoID,
		IsPrivate: rel.Repo.IsPrivate,
		Content:   rel.Title,
		RefName:   rel.TagName, // FIXME: use a full ref name?
	})
}

// ... later decoded in models/activities/action.go:GetIssueInfos
func encodeContent(params ...string) string {
	contentEncoded, err := json.Marshal(params)
	if err != nil {
		log.Error("encodeContent: Unexpected json encoding error: %v", err)
	}
	return string(contentEncoded)
}

// Given a comment of arbitrary-length Markdown text, create an abbreviated Markdown text appropriate for the
// activity feed.
func abbreviatedComment(comment string) string {
	firstLine := strings.Split(comment, "\n")[0]

	if strings.HasPrefix(firstLine, "```") {
		// First line is is a fenced code block... with no special abbreviate we would display a blank block, or in the
		// worst-case a ```mermaid would display an error. Better to omit the comment.
		return ""
	}

	truncatedContent, truncatedRight := util.SplitStringAtByteN(firstLine, 200)
	if truncatedRight != "" {
		// in case the content is in a Latin family language, we remove the last broken word.
		lastSpaceIdx := strings.LastIndex(truncatedContent, " ")
		if lastSpaceIdx != -1 && (len(truncatedContent)-lastSpaceIdx < 15) {
			truncatedContent = truncatedContent[:lastSpaceIdx] + "…"
		}
	}

	return truncatedContent
}

// Return a clone of the incoming repository.PushCommits that is appropriately tweaked for the activity feed. The struct
// is cloned rather than modified in-place because the same data will be sent to multiple notifiers. Transformations
// applied are: # of commits are limited to FeedMaxCommitNum, commit messages are trimmed to just the content displayed
// in the activity feed.
func prepareCommitsForFeed(commits *repository.PushCommits) *repository.PushCommits {
	numCommits := min(len(commits.Commits), setting.UI.FeedMaxCommitNum)
	retval := repository.PushCommits{
		Commits:    make([]*repository.PushCommit, 0, numCommits),
		HeadCommit: nil,
		CompareURL: commits.CompareURL,
		Len:        commits.Len,
	}
	if commits.HeadCommit != nil {
		retval.HeadCommit = prepareCommitForFeed(commits.HeadCommit)
	}
	for i, commit := range commits.Commits {
		if i == numCommits {
			break
		}
		retval.Commits = append(retval.Commits, prepareCommitForFeed(commit))
	}
	return &retval
}

func prepareCommitForFeed(commit *repository.PushCommit) *repository.PushCommit {
	return &repository.PushCommit{
		Sha1:           commit.Sha1,
		Message:        abbreviatedComment(commit.Message),
		AuthorEmail:    commit.AuthorEmail,
		AuthorName:     commit.AuthorName,
		CommitterEmail: commit.CommitterEmail,
		CommitterName:  commit.CommitterName,
		Signature:      commit.Signature,
		Verification:   commit.Verification,
		Timestamp:      commit.Timestamp,
	}
}
