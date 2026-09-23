// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package webhook

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/git"
	"forgejo.org/modules/json"
	"forgejo.org/modules/repository"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"
	webhook_module "forgejo.org/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pushCommits() *repository.PushCommits {
	pushCommits := repository.NewPushCommits()
	pushCommits.Commits = []*repository.PushCommit{
		{
			Sha1:           "2c54faec6c45d31c1abfaecdab471eac6633738a",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "not signed commit",
		},
		{
			Sha1:           "205ac761f3326a7ebe416e8673760016450b5cec",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "good signed commit (with not yet validated email)",
		},
		{
			Sha1:           "1032bbf17fbc0d9c95bb5418dabe8f8c99278700",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "good signed commit",
		},
	}
	pushCommits.HeadCommit = &repository.PushCommit{Sha1: "2c54faec6c45d31c1abfaecdab471eac6633738a"}
	return pushCommits
}

func TestSyncPushCommits(t *testing.T) {
	defer unittest.OverrideFixtures("services/webhook/TestPushCommits")()
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2, OwnerID: user.ID})

	t.Run("All commits", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

		NewNotifier().SyncPushCommits(db.DefaultContext, user, repo, &repository.PushUpdateOptions{RefFullName: git.RefNameFromBranch("master-1")}, pushCommits())

		hookTask := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{}, unittest.Cond("payload_content LIKE '%master-1%'"))

		var payloadContent structs.PushPayload
		require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))
		assert.Len(t, payloadContent.Commits, 3)
	})

	t.Run("Only one commit", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 1)()

		NewNotifier().SyncPushCommits(db.DefaultContext, user, repo, &repository.PushUpdateOptions{RefFullName: git.RefNameFromBranch("main-1")}, pushCommits())

		hookTask := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{}, unittest.Cond("payload_content LIKE '%main-1%'"))

		var payloadContent structs.PushPayload
		require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))
		assert.Len(t, payloadContent.Commits, 1)
		assert.Equal(t, "2c54faec6c45d31c1abfaecdab471eac6633738a", payloadContent.Commits[0].ID)
	})
}

func TestPushCommits(t *testing.T) {
	defer unittest.OverrideFixtures("services/webhook/TestPushCommits")()
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2, OwnerID: user.ID})

	t.Run("All commits", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

		NewNotifier().PushCommits(db.DefaultContext, user, repo, &repository.PushUpdateOptions{RefFullName: git.RefNameFromBranch("master-2")}, pushCommits())

		hookTask := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{}, unittest.Cond("payload_content LIKE '%master-2%'"))

		var payloadContent structs.PushPayload
		require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))
		assert.Len(t, payloadContent.Commits, 3)
	})

	t.Run("Only one commit", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 1)()

		NewNotifier().PushCommits(db.DefaultContext, user, repo, &repository.PushUpdateOptions{RefFullName: git.RefNameFromBranch("main-2")}, pushCommits())

		hookTask := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{}, unittest.Cond("payload_content LIKE '%main-2%'"))

		var payloadContent structs.PushPayload
		require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))
		assert.Len(t, payloadContent.Commits, 1)
		assert.Equal(t, "2c54faec6c45d31c1abfaecdab471eac6633738a", payloadContent.Commits[0].ID)
	})
}

func assertActionEqual(t *testing.T, expectedRun *actions_model.ActionRun, actualRun *structs.ActionRun) {
	assert.NotNil(t, expectedRun)
	assert.NotNil(t, actualRun)
	// only test a few things
	assert.Equal(t, expectedRun.ID, actualRun.ID)
	assert.Equal(t, expectedRun.Status.String(), actualRun.Status)
	assert.Equal(t, expectedRun.Index, actualRun.Index)
	assert.Equal(t, expectedRun.RepoID, actualRun.Repo.ID)
	// convert to unix because of time zones
	assert.Equal(t, expectedRun.Stopped.AsTime().Unix(), actualRun.Stopped.ValueOrZeroValue().Unix())
	assert.Equal(t, expectedRun.Title, actualRun.Title)
	assert.Equal(t, expectedRun.WorkflowID, actualRun.WorkflowID)
}

func TestAction(t *testing.T) {
	defer unittest.OverrideFixtures("services/webhook/TestPushCommits")()
	require.NoError(t, unittest.PrepareTestDatabase())

	triggerUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2, OwnerID: triggerUser.ID})

	newSuccessRun := &actions_model.ActionRun{
		ID:            1,
		Status:        actions_model.StatusSuccess,
		Index:         1,
		RepoID:        repo.ID,
		Stopped:       1693648327,
		WorkflowID:    "some_workflow",
		Title:         "newSuccessRun",
		TriggerUser:   triggerUser,
		TriggerUserID: triggerUser.ID,
		TriggerEvent:  "push",
	}
	newSuccessRun.LoadAttributes(db.DefaultContext)
	newFailureRun := &actions_model.ActionRun{
		ID:            1,
		Status:        actions_model.StatusFailure,
		Index:         1,
		RepoID:        repo.ID,
		Stopped:       1693648327,
		WorkflowID:    "some_workflow",
		Title:         "newFailureRun",
		TriggerUser:   triggerUser,
		TriggerUserID: triggerUser.ID,
		TriggerEvent:  "push",
	}
	newFailureRun.LoadAttributes(db.DefaultContext)

	t.Run("Successful Run", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

		NewNotifier().WorkflowRunCompleted(t.Context(), newSuccessRun, actions_model.StatusWaiting)

		// there's only one of these at the time
		hookTask := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{}, unittest.Cond("event_type == 'action_run_success' AND payload_content LIKE '%success%newSuccessRun%'"))
		assert.Equal(t, webhook_module.HookEventActionRunSuccess, hookTask.EventType)

		var payloadContent structs.ActionPayload
		require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))
		assert.Equal(t, structs.HookActionSuccess, payloadContent.Action)
		assert.Equal(t, actions_model.StatusWaiting.String(), payloadContent.PriorStatus)
		assertActionEqual(t, newSuccessRun, payloadContent.Run)
	})

	t.Run("Failed Run", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

		NewNotifier().WorkflowRunCompleted(t.Context(), newFailureRun, actions_model.StatusWaiting)

		// there should only be this one at the time
		hookTask := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{}, unittest.Cond("event_type == 'action_run_failure' AND payload_content LIKE '%failure%newFailureRun%'"))
		assert.Equal(t, webhook_module.HookEventActionRunFailure, hookTask.EventType)

		var payloadContent structs.ActionPayload
		require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))
		assert.Equal(t, structs.HookActionFailure, payloadContent.Action)
		assert.Equal(t, actions_model.StatusWaiting.String(), payloadContent.PriorStatus)
		assertActionEqual(t, newFailureRun, payloadContent.Run)
	})
}

func TestWebhookNotifier_NewWorkflowRunAttempt(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})

	webhook := webhook_model.Webhook{
		OwnerID:     user2.ID,
		RepoID:      repo62.ID,
		URL:         "https://example.com/",
		HTTPMethod:  "POST",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"send_everything":true}`,
		IsActive:    true,
		Type:        webhook_module.FORGEJO,
	}

	unittest.AssertSuccessfulInsert(t, webhook)

	run := &actions_model.ActionRun{
		Title:       "Update package.json",
		RepoID:      repo62.ID,
		OwnerID:     user2.ID,
		TriggerUser: user2,
		Status:      actions_model.StatusWaiting,
		Index:       29,
	}

	unittest.AssertSuccessfulInsert(t, run)

	require.NoError(t, run.LoadAttributes(t.Context()))

	notifier := webhookNotifier{}
	notifier.NewWorkflowRunAttempt(t.Context(), run)

	hookTask := unittest.AssertExistsAndLoadBean(t,
		&webhook_model.HookTask{EventType: webhook_module.HookEventWorkflowRunWaiting})

	var payloadContent structs.WorkflowRunPayload
	require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))

	assert.Equal(t, structs.HookNewWorkflowRunAttempt, payloadContent.Action)
	assert.Equal(t, run.ID, payloadContent.Run.ID)
	assert.Equal(t, repo62.ID, payloadContent.Run.Repo.ID)
}

func TestWebhookNotifier_WorkflowRunStatusChanged(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})

	webhook := webhook_model.Webhook{
		OwnerID:     user2.ID,
		RepoID:      repo62.ID,
		URL:         "https://example.com/",
		HTTPMethod:  "POST",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"send_everything":true}`,
		IsActive:    true,
		Type:        webhook_module.FORGEJO,
	}

	unittest.AssertSuccessfulInsert(t, webhook)

	run := &actions_model.ActionRun{
		Title:       "Update package.json",
		RepoID:      repo62.ID,
		OwnerID:     user2.ID,
		TriggerUser: user2,
		Status:      actions_model.StatusRunning,
		Index:       29,
	}

	unittest.AssertSuccessfulInsert(t, run)

	require.NoError(t, run.LoadAttributes(t.Context()))

	notifier := webhookNotifier{}
	notifier.WorkflowRunStatusChanged(t.Context(), run, actions_model.StatusWaiting)

	hookTask := unittest.AssertExistsAndLoadBean(t,
		&webhook_model.HookTask{EventType: webhook_module.HookEventWorkflowRunRunning})

	var payloadContent structs.WorkflowRunPayload
	require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))

	assert.Equal(t, structs.HookWorkflowRunStatusChanged, payloadContent.Action)
	assert.Equal(t, run.ID, payloadContent.Run.ID)
	assert.Equal(t, repo62.ID, payloadContent.Run.Repo.ID)
}

func TestWebhookNotifier_WorkflowRunCompleted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})

	webhook := webhook_model.Webhook{
		OwnerID:     user2.ID,
		RepoID:      repo62.ID,
		URL:         "https://example.com/",
		HTTPMethod:  "POST",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"send_everything":true}`,
		IsActive:    true,
		Type:        webhook_module.FORGEJO,
	}

	unittest.AssertSuccessfulInsert(t, webhook)

	run := &actions_model.ActionRun{
		Title:       "Update package.json",
		RepoID:      repo62.ID,
		OwnerID:     user2.ID,
		TriggerUser: user2,
		Status:      actions_model.StatusCancelled,
		Index:       29,
	}

	unittest.AssertSuccessfulInsert(t, run)

	require.NoError(t, run.LoadAttributes(t.Context()))

	notifier := webhookNotifier{}
	notifier.WorkflowRunCompleted(t.Context(), run, actions_model.StatusRunning)

	hookTask := unittest.AssertExistsAndLoadBean(t,
		&webhook_model.HookTask{EventType: webhook_module.HookEventWorkflowRunCancelled})

	var payloadContent structs.WorkflowRunPayload
	require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))

	assert.Equal(t, structs.HookWorkflowRunCompleted, payloadContent.Action)
	assert.Equal(t, run.ID, payloadContent.Run.ID)
	assert.Equal(t, repo62.ID, payloadContent.Run.Repo.ID)
}

func TestWebhookNotifier_NewWorkflowJobAttempt(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})

	webhook := webhook_model.Webhook{
		OwnerID:     user2.ID,
		RepoID:      repo62.ID,
		URL:         "https://example.com/",
		HTTPMethod:  "POST",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"send_everything":true}`,
		IsActive:    true,
		Type:        webhook_module.FORGEJO,
	}

	unittest.AssertSuccessfulInsert(t, webhook)

	run := &actions_model.ActionRun{
		Title:       "Update pom.xml",
		RepoID:      repo62.ID,
		OwnerID:     user2.ID,
		TriggerUser: user2,
		Status:      actions_model.StatusWaiting,
	}

	unittest.AssertSuccessfulInsert(t, run)

	job := &actions_model.ActionRunJob{
		RunID:             run.ID,
		RepoID:            repo62.ID,
		OwnerID:           user2.ID,
		CommitSHA:         "365cc67e3d824b0c8dabf6f7799990ed2a9ce271",
		IsForkPullRequest: false,
		Name:              "build",
		Attempt:           2,
		Handle:            "51ddf9d1-2649-4b08-9fde-6867275a4b28",
		JobID:             "build",
		RunsOn:            []string{"fedora", "size-m"},
		Status:            actions_model.StatusWaiting,
		Started:           0,
		Stopped:           0,
		Created:           timeutil.TimeStamp(1789052551),
		Updated:           timeutil.TimeStamp(1789052552),
	}

	unittest.AssertSuccessfulInsert(t, job)

	require.NoError(t, job.LoadAttributes(t.Context()))

	notifier := webhookNotifier{}
	notifier.NewWorkflowJobAttempt(t.Context(), job)

	hookTask := unittest.AssertExistsAndLoadBean(t,
		&webhook_model.HookTask{EventType: webhook_module.HookEventWorkflowJobWaiting})

	var payloadContent structs.WorkflowJobPayload
	require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))

	assert.Equal(t, structs.HookNewWorkflowJobAttempt, payloadContent.Action)
	assert.Equal(t, job.ID, payloadContent.Job.ID)
	assert.Equal(t, run.ID, payloadContent.Run.ID)
	assert.Equal(t, repo62.ID, payloadContent.Run.Repo.ID)
}

func TestWebhookNotifier_WorkflowJobStatusChanged(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})

	webhook := webhook_model.Webhook{
		OwnerID:     user2.ID,
		RepoID:      repo62.ID,
		URL:         "https://example.com/",
		HTTPMethod:  "POST",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"send_everything":true}`,
		IsActive:    true,
		Type:        webhook_module.FORGEJO,
	}

	unittest.AssertSuccessfulInsert(t, webhook)

	run := &actions_model.ActionRun{
		Title:       "Update pom.xml",
		RepoID:      repo62.ID,
		OwnerID:     user2.ID,
		TriggerUser: user2,
		Status:      actions_model.StatusRunning,
	}

	unittest.AssertSuccessfulInsert(t, run)

	job := &actions_model.ActionRunJob{
		RunID:             run.ID,
		RepoID:            repo62.ID,
		OwnerID:           user2.ID,
		CommitSHA:         "365cc67e3d824b0c8dabf6f7799990ed2a9ce271",
		IsForkPullRequest: false,
		Name:              "build",
		Attempt:           2,
		Handle:            "51ddf9d1-2649-4b08-9fde-6867275a4b28",
		JobID:             "build",
		RunsOn:            []string{"fedora", "size-m"},
		Status:            actions_model.StatusRunning,
		Started:           1789052552,
		Stopped:           0,
		Created:           timeutil.TimeStamp(1789052551),
		Updated:           timeutil.TimeStamp(1789052552),
	}

	unittest.AssertSuccessfulInsert(t, job)

	require.NoError(t, job.LoadAttributes(t.Context()))

	notifier := webhookNotifier{}
	notifier.WorkflowJobStatusChanged(t.Context(), job, actions_model.StatusWaiting)

	hookTask := unittest.AssertExistsAndLoadBean(t,
		&webhook_model.HookTask{EventType: webhook_module.HookEventWorkflowJobRunning})

	var payloadContent structs.WorkflowJobPayload
	require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))

	assert.Equal(t, structs.HookWorkflowJobStatusChanged, payloadContent.Action)
	assert.Equal(t, job.ID, payloadContent.Job.ID)
	assert.Equal(t, run.ID, payloadContent.Run.ID)
	assert.Equal(t, repo62.ID, payloadContent.Run.Repo.ID)
}

func TestWebhookNotifier_WorkflowJobCompleted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	defer test.MockVariableValue(&setting.Webhook.PayloadCommitLimit, 10)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})

	webhook := webhook_model.Webhook{
		OwnerID:     user2.ID,
		RepoID:      repo62.ID,
		URL:         "https://example.com/",
		HTTPMethod:  "POST",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"send_everything":true}`,
		IsActive:    true,
		Type:        webhook_module.FORGEJO,
	}

	unittest.AssertSuccessfulInsert(t, webhook)

	run := &actions_model.ActionRun{
		Title:       "Update pom.xml",
		RepoID:      repo62.ID,
		OwnerID:     user2.ID,
		TriggerUser: user2,
		Status:      actions_model.StatusSuccess,
	}

	unittest.AssertSuccessfulInsert(t, run)

	job := &actions_model.ActionRunJob{
		RunID:             run.ID,
		RepoID:            repo62.ID,
		OwnerID:           user2.ID,
		CommitSHA:         "365cc67e3d824b0c8dabf6f7799990ed2a9ce271",
		IsForkPullRequest: false,
		Name:              "build",
		Attempt:           2,
		Handle:            "51ddf9d1-2649-4b08-9fde-6867275a4b28",
		JobID:             "build",
		RunsOn:            []string{"fedora", "size-m"},
		Status:            actions_model.StatusSuccess,
		Started:           1789052552,
		Stopped:           1789052573,
		Created:           timeutil.TimeStamp(1789052551),
		Updated:           timeutil.TimeStamp(1789052574),
	}

	unittest.AssertSuccessfulInsert(t, job)

	require.NoError(t, job.LoadAttributes(t.Context()))

	notifier := webhookNotifier{}
	notifier.WorkflowJobCompleted(t.Context(), job, actions_model.StatusRunning)

	hookTask := unittest.AssertExistsAndLoadBean(t,
		&webhook_model.HookTask{EventType: webhook_module.HookEventWorkflowJobSuccess})

	var payloadContent structs.WorkflowJobPayload
	require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payloadContent))

	assert.Equal(t, structs.HookWorkflowJobCompleted, payloadContent.Action)
	assert.Equal(t, job.ID, payloadContent.Job.ID)
	assert.Equal(t, run.ID, payloadContent.Run.ID)
	assert.Equal(t, repo62.ID, payloadContent.Run.Repo.ID)
}
