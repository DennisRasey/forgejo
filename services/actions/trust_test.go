// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	issues_model "forgejo.org/models/issues"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	webhook_module "forgejo.org/modules/webhook"

	gouuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsTrust_ChangeStatus(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(10)
	pullRequestPosterID := int64(30)

	runDone := &actions_model.ActionRun{
		RepoID:              repoID,
		PullRequestPosterID: pullRequestPosterID,
		Status:              actions_model.StatusSuccess,
	}
	require.NoError(t, actions_model.InsertRunWithoutNotification(t.Context(), runDone, nil))

	runNotByPoster := &actions_model.ActionRun{
		RepoID:              repoID,
		PullRequestPosterID: 43243,
		Status:              actions_model.StatusRunning,
	}
	require.NoError(t, actions_model.InsertRunWithoutNotification(t.Context(), runNotByPoster, nil))

	runNotInTheSameRepository := &actions_model.ActionRun{
		RepoID:              5,
		PullRequestPosterID: pullRequestPosterID,
		Status:              actions_model.StatusSuccess,
	}
	require.NoError(t, actions_model.InsertRunWithoutNotification(t.Context(), runNotInTheSameRepository, nil))

	t.Run("RevokeTrust", func(t *testing.T) {
		runNotDone := &actions_model.ActionRun{
			TriggerUserID:       2,
			RepoID:              repoID,
			Status:              actions_model.StatusWaiting,
			PullRequestPosterID: pullRequestPosterID,
		}
		waitingJob := &actions_model.ActionRunJob{
			Status: actions_model.StatusWaiting,
			Handle: gouuid.New().String(),
		}
		jobs := []*actions_model.ActionRunJob{waitingJob}
		require.NoError(t, actions_model.InsertRunWithoutNotification(t.Context(), runNotDone, jobs))
		require.NoError(t, actions_model.InsertActionUser(t.Context(), &actions_model.ActionUser{
			UserID:                  pullRequestPosterID,
			RepoID:                  repoID,
			TrustedWithPullRequests: true,
		}))

		previousCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})
		_, err := actions_model.GetActionUserByUserIDAndRepoIDAndUpdateAccess(t.Context(), pullRequestPosterID, repoID)
		require.NoError(t, err)

		require.NoError(t, RevokeTrust(t.Context(), repoID, pullRequestPosterID))

		_, err = actions_model.GetActionUserByUserIDAndRepoIDAndUpdateAccess(t.Context(), pullRequestPosterID, repoID)
		assert.True(t, actions_model.IsErrUserNotExist(err))
		currentCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})
		assert.Equal(t, previousCancelledCount+1, currentCancelledCount)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runNotDone.ID})
		assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
	})

	createPullRequestRun := func(t *testing.T, pullRequestID, repoID int64) *actions_model.ActionRun {
		t.Helper()
		runNotApproved := &actions_model.ActionRun{
			TriggerUserID:       2,
			RepoID:              repoID,
			Status:              actions_model.StatusBlocked,
			NeedApproval:        true,
			PullRequestID:       pullRequestID,
			PullRequestPosterID: pullRequestPosterID,
		}
		waitingJob := &actions_model.ActionRunJob{Status: actions_model.StatusBlocked, Handle: gouuid.New().String()}
		jobs := []*actions_model.ActionRunJob{waitingJob}
		require.NoError(t, actions_model.InsertRunWithoutNotification(t.Context(), runNotApproved, jobs))
		return runNotApproved
	}

	t.Run("PullRequestCancel", func(t *testing.T) {
		pullRequestID := int64(485)
		runNotApproved := createPullRequestRun(t, pullRequestID, repoID)

		previousCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})

		require.NoError(t, pullRequestCancel(t.Context(), repoID, pullRequestID))

		currentCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})
		assert.Equal(t, previousCancelledCount+1, currentCancelledCount)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runNotApproved.ID})
		assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
	})

	t.Run("UpdateTrustedWithPullRequest deny", func(t *testing.T) {
		pullRequestID := int64(485)
		runNotApproved := createPullRequestRun(t, pullRequestID, repoID)

		previousCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})

		require.NoError(t, UpdateTrustedWithPullRequest(t.Context(), 0, &issues_model.PullRequest{
			ID: pullRequestID,
			Issue: &issues_model.Issue{
				RepoID: repoID,
			},
		}, UserTrustDenied))

		currentCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})
		assert.Equal(t, previousCancelledCount+1, currentCancelledCount)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runNotApproved.ID})
		assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
	})

	t.Run("PullRequestApprove", func(t *testing.T) {
		pullRequestID := int64(534)
		runNotApproved := createPullRequestRun(t, pullRequestID, repoID)

		previousWaitingCount := unittest.GetCount(t, &actions_model.ActionRunJob{Status: actions_model.StatusWaiting})

		doerID := int64(84322)
		require.NoError(t, pullRequestApprove(t.Context(), doerID, repoID, pullRequestID))

		currentWaitingCount := unittest.GetCount(t, &actions_model.ActionRunJob{Status: actions_model.StatusWaiting})
		assert.Equal(t, previousWaitingCount+1, currentWaitingCount)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runNotApproved.ID})
		assert.Equal(t, actions_model.StatusWaiting.String(), run.Status.String())
		assert.Equal(t, doerID, run.ApprovedBy)
		assert.False(t, run.NeedApproval)
	})

	t.Run("UpdateTrustedWithPullRequest once", func(t *testing.T) {
		pullRequestID := int64(534)
		runNotApproved := createPullRequestRun(t, pullRequestID, repoID)

		previousWaitingCount := unittest.GetCount(t, &actions_model.ActionRunJob{Status: actions_model.StatusWaiting})

		doerID := int64(84322)
		require.NoError(t, UpdateTrustedWithPullRequest(t.Context(), doerID, &issues_model.PullRequest{
			ID: pullRequestID,
			Issue: &issues_model.Issue{
				RepoID: repoID,
			},
		}, UserTrustedOnce))

		currentWaitingCount := unittest.GetCount(t, &actions_model.ActionRunJob{Status: actions_model.StatusWaiting})
		assert.Equal(t, previousWaitingCount+1, currentWaitingCount)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runNotApproved.ID})
		assert.Equal(t, actions_model.StatusWaiting.String(), run.Status.String())
		assert.Equal(t, doerID, run.ApprovedBy)
		assert.False(t, run.NeedApproval)
	})

	t.Run("UpdateTrustedWithPullRequest always", func(t *testing.T) {
		pullRequestIDs := []int64{534, 645}
		var runsNotApproved []*actions_model.ActionRun
		for _, pullRequestID := range pullRequestIDs {
			runsNotApproved = append(runsNotApproved, createPullRequestRun(t, pullRequestID, repoID))
		}

		previousWaitingCount := unittest.GetCount(t, &actions_model.ActionRunJob{Status: actions_model.StatusWaiting})

		doerID := int64(84322)
		require.NoError(t, UpdateTrustedWithPullRequest(t.Context(), doerID, &issues_model.PullRequest{
			ID: pullRequestIDs[0],
			Issue: &issues_model.Issue{
				RepoID:   repoID,
				PosterID: pullRequestPosterID,
			},
		}, UserAlwaysTrusted))

		currentWaitingCount := unittest.GetCount(t, &actions_model.ActionRunJob{Status: actions_model.StatusWaiting})
		assert.Equal(t, previousWaitingCount+len(pullRequestIDs), currentWaitingCount)

		for _, run := range runsNotApproved {
			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
			assert.Equal(t, actions_model.StatusWaiting.String(), run.Status.String())
			assert.Equal(t, doerID, run.ApprovedBy)
			assert.False(t, run.NeedApproval)
		}
	})

	t.Run("UpdateTrustedWithPullRequest revoke", func(t *testing.T) {
		pullRequestIDs := []int64{748, 953}
		var runsNotApproved []*actions_model.ActionRun
		for _, pullRequestID := range pullRequestIDs {
			runsNotApproved = append(runsNotApproved, createPullRequestRun(t, pullRequestID, repoID))
		}

		doerID := int64(84322)
		require.NoError(t, UpdateTrustedWithPullRequest(t.Context(), doerID, &issues_model.PullRequest{
			ID: pullRequestIDs[0],
			Issue: &issues_model.Issue{
				RepoID:   repoID,
				PosterID: pullRequestPosterID,
			},
		}, UserTrustRevoked))

		for _, run := range runsNotApproved {
			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
			assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
			assert.False(t, run.NeedApproval)
		}
	})

	t.Run("cleanupPullRequestUnapprovedRuns", func(t *testing.T) {
		pullRequestID := int64(534)
		runNotApproved := createPullRequestRun(t, pullRequestID, repoID)

		previousCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})

		require.NoError(t, cleanupPullRequestUnapprovedRuns(t.Context(), repoID, pullRequestID))

		currentCancelledCount := unittest.GetCount(t, &actions_model.ActionRun{Status: actions_model.StatusCancelled})
		assert.Equal(t, previousCancelledCount+1, currentCancelledCount)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runNotApproved.ID})
		assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
	})
}

func TestActionsTrust_GetPullRequestUserIsTrustedWithActions(t *testing.T) {
	defer unittest.OverrideFixtures("services/actions/TestActionsTrust_GetPullRequestUserIsTrustedWithActions")()
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("implicitly trusted because the pull request is not from a fork", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2000})
		trust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.NoError(t, err)
		require.False(t, pr.IsForkPullRequest())
		assert.Equal(t, UserIsImplicitlyTrustedWithActions, trust)
	})

	t.Run("implicitly trusted on a forked pull request when the poster is admin", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3000})
		trust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.NoError(t, err)
		require.True(t, pr.IsForkPullRequest())
		require.True(t, pr.Issue.Poster.IsAdmin)
		assert.Equal(t, UserIsImplicitlyTrustedWithActions, trust)
	})

	t.Run("explicitly trusted on a forked pull request when the poster was permanently approved", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1000})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // regular user
		trust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.NoError(t, err)
		require.True(t, pr.IsForkPullRequest())
		_, err = actions_model.GetActionUserByUserIDAndRepoIDAndUpdateAccess(t.Context(), user.ID, pr.Issue.RepoID)
		require.NoError(t, err)
		assert.Equal(t, UserIsExplicitlyTrustedWithActions, trust)
	})

	t.Run("not trusted because on a forked pull request when the user has has no privileges", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 4000})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}) // regular user
		trust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.NoError(t, err)
		assert.Equal(t, user.ID, pr.Issue.PosterID)
		require.True(t, pr.IsForkPullRequest())
		assert.Equal(t, UserIsNotTrustedWithActions, trust)
	})

	t.Run("not trusted on a forked pull request because the user is restricted", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 5000})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29}) // restricted user
		trust, err := getPullRequestUserIsTrustedWithActions(t.Context(), pr, user)
		require.NoError(t, err)
		assert.Equal(t, user.ID, pr.Issue.PosterID)
		require.True(t, pr.IsForkPullRequest())
		_, err = actions_model.GetActionUserByUserIDAndRepoIDAndUpdateAccess(t.Context(), user.ID, pr.Issue.RepoID)
		require.NoError(t, err)
		require.True(t, user.IsRestricted)
		assert.Equal(t, UserIsNotTrustedWithActions, trust)
	})

	t.Run("approval not needed because the pr is not from a fork", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2000})
		useCommit, approval, err := getPullRequestCommitAndApproval(t.Context(), pr, nil, webhook_module.HookEventPullRequest)
		require.NoError(t, err)
		assert.Equal(t, actions_model.DoesNotNeedApproval, approval)
		assert.EqualValues(t, useHeadCommit, useCommit)
	})

	t.Run("approval not needed because the event is known to run out of the default branch", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3000})
		useCommit, approval, err := getPullRequestCommitAndApproval(t.Context(), pr, nil, webhook_module.HookEventPullRequestComment)
		require.NoError(t, err)
		assert.Equal(t, actions_model.DoesNotNeedApproval, approval)
		assert.EqualValues(t, useHeadCommit, useCommit)
	})

	t.Run("approval not needed because it is not a pr", func(t *testing.T) {
		useCommit, approval, err := getPullRequestCommitAndApproval(t.Context(), nil, nil, webhook_module.HookEventPullRequestComment)
		require.NoError(t, err)
		assert.Equal(t, actions_model.DoesNotNeedApproval, approval)
		assert.EqualValues(t, useHeadCommit, useCommit)
	})

	t.Run("approval not needed for a forked pr because the poster is trusted", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3000})
		useCommit, approval, err := getPullRequestCommitAndApproval(t.Context(), pr, nil, webhook_module.HookEventPullRequestSync)
		require.NoError(t, err)
		require.True(t, pr.Issue.Poster.IsAdmin)
		assert.Equal(t, actions_model.DoesNotNeedApproval, approval)
		assert.EqualValues(t, useHeadCommit, useCommit)
	})

	t.Run("approval needed for a forked pr because the poster and the doer are not trusted", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 4000})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}) // regular user
		useCommit, approval, err := getPullRequestCommitAndApproval(t.Context(), pr, doer, webhook_module.HookEventPullRequestSync)
		require.NoError(t, err)
		posterTrust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.NoError(t, err)
		require.Equal(t, UserIsNotTrustedWithActions, posterTrust)
		doerTrust, err := getPullRequestUserIsTrustedWithActions(t.Context(), pr, doer)
		require.NoError(t, err)
		require.Equal(t, UserIsNotTrustedWithActions, doerTrust)
		assert.Equal(t, actions_model.NeedApproval, approval)
		assert.EqualValues(t, useHeadCommit, useCommit)
	})

	t.Run("approval not needed for a forked pr because the doer is trusted and runs from the base", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 4000})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}) // admin
		useCommit, approval, err := getPullRequestCommitAndApproval(t.Context(), pr, doer, webhook_module.HookEventPullRequestLabel)
		require.NoError(t, err)
		posterTrust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.NoError(t, err)
		require.Equal(t, UserIsNotTrustedWithActions, posterTrust)
		doerTrust, err := getPullRequestUserIsTrustedWithActions(t.Context(), pr, doer)
		require.NoError(t, err)
		require.Equal(t, UserIsImplicitlyTrustedWithActions, doerTrust)
		assert.Equal(t, actions_model.DoesNotNeedApproval, approval)
		assert.EqualValues(t, useBaseCommit, useCommit)
	})

	t.Run("approval not needed for a forked pr because the doer is trusted and pushed new commits", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 4000})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}) // admin
		useCommit, approval, err := getPullRequestCommitAndApproval(t.Context(), pr, doer, webhook_module.HookEventPullRequestSync)
		require.NoError(t, err)
		posterTrust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.NoError(t, err)
		require.Equal(t, UserIsNotTrustedWithActions, posterTrust)
		doerTrust, err := getPullRequestUserIsTrustedWithActions(t.Context(), pr, doer)
		require.NoError(t, err)
		require.Equal(t, UserIsImplicitlyTrustedWithActions, doerTrust)
		assert.Equal(t, actions_model.DoesNotNeedApproval, approval)
		assert.EqualValues(t, useHeadCommit, useCommit)
	})

	t.Run("run for a pull request is set with info related to trust", func(t *testing.T) {
		run := &actions_model.ActionRun{
			IsForkPullRequest: true,
		}
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 5000})
		needApproval := actions_model.NeedApproval
		require.NoError(t, setRunTrustForPullRequest(t.Context(), run, nil, needApproval))
		require.NoError(t, setRunTrustForPullRequest(t.Context(), run, pr, needApproval))
		assert.True(t, run.NeedApproval)
		assert.True(t, run.IsForkPullRequest)
		assert.Equal(t, pr.Issue.PosterID, run.PullRequestPosterID)
		assert.Equal(t, pr.ID, run.PullRequestID)
	})

	t.Run("no poster set on issue linked to pull request", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 6000})
		posterTrust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.ErrorIs(t, err, issues_model.ErrNoPosterSetOnIssue)
		require.Equal(t, UserIsNotTrustedWithActions, posterTrust)
	})

	t.Run("poster set on issue linked to pull request does not exist", func(t *testing.T) {
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 7000})
		posterTrust, err := GetPullRequestPosterIsTrustedWithActions(t.Context(), pr)
		require.ErrorIs(t, err, user_model.ErrUserNotExist{UID: 666})
		require.Equal(t, UserIsNotTrustedWithActions, posterTrust)
	})
}
