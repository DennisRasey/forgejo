// Copyright 2024 The Forgejo Authors
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"slices"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	actions_module "forgejo.org/modules/actions"
	"forgejo.org/modules/git"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	webhook_module "forgejo.org/modules/webhook"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
	"code.forgejo.org/forgejo/runner/v13/act/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsNotifier_SkipPullRequestEvent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(1)
	commitSHA := "1234"

	// event is not webhook_module.HookEventPullRequestSync, never skip
	assert.False(t, SkipPullRequestEvent(db.DefaultContext, webhook_module.HookEventPullRequest, repoID, commitSHA))

	// event is webhook_module.HookEventPullRequestSync but there is nothing in the ActionRun table, do not skip
	assert.False(t, SkipPullRequestEvent(db.DefaultContext, webhook_module.HookEventPullRequestSync, repoID, commitSHA))

	// there is a webhook_module.HookEventPullRequest event but the SHA is different, do not skip
	index := int64(1)
	run := &actions_model.ActionRun{
		Index:     index,
		Event:     webhook_module.HookEventPullRequest,
		RepoID:    repoID,
		CommitSHA: "othersha",
	}
	unittest.AssertSuccessfulInsert(t, run)
	assert.False(t, SkipPullRequestEvent(db.DefaultContext, webhook_module.HookEventPullRequestSync, repoID, commitSHA))

	// there already is a webhook_module.HookEventPullRequest with the same SHA, skip
	index++
	run = &actions_model.ActionRun{
		Index:     index,
		Event:     webhook_module.HookEventPullRequest,
		RepoID:    repoID,
		CommitSHA: commitSHA,
	}
	unittest.AssertSuccessfulInsert(t, run)
	assert.True(t, SkipPullRequestEvent(db.DefaultContext, webhook_module.HookEventPullRequestSync, repoID, commitSHA))
}

func TestActionsNotifier_IssueCommentOnForkPullRequestEvent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
	require.NoError(t, pr.LoadIssue(db.DefaultContext))

	require.True(t, pr.IsFromFork())

	commit := &git.Commit{
		ID:            git.MustIDFromString("0000000000000000000000000000000000000000"),
		CommitMessage: "test",
	}
	detectedWorkflows := []*actions_module.DetectedWorkflow{
		{
			TriggerEvent: &jobparser.Event{
				Name: "issue_comment",
			},
		},
	}
	input := &NotifyInput{
		Repo:        repo,
		Doer:        doer,
		Event:       webhook_module.HookEventIssueComment,
		PullRequest: pr,
		Payload:     &api.IssueCommentPayload{},
	}

	unittest.AssertSuccessfulDelete(t, &actions_model.ActionRun{RepoID: repo.ID})

	err := handleWorkflows(db.DefaultContext, detectedWorkflows, commit, input, "")
	require.NoError(t, err)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)

	assert.Equal(t, webhook_module.HookEventIssueComment, runs[0].Event)
	assert.False(t, runs[0].IsForkPullRequest)
}

func testActionsNotifierPullRequest(t *testing.T, repo *repo_model.Repository, pr *issues_model.PullRequest, dw *actions_module.DetectedWorkflow, event webhook_module.HookEventType) {
	t.Helper()

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	require.NoError(t, pr.LoadIssue(db.DefaultContext))

	testActionsNotifierPullRequestWithDoer(t, repo, pr, doer, dw, event)
}

func testActionsNotifierPullRequestWithDoer(t *testing.T, repo *repo_model.Repository, pr *issues_model.PullRequest, doer *user_model.User, dw *actions_module.DetectedWorkflow, event webhook_module.HookEventType) {
	t.Helper()

	commit := &git.Commit{
		ID:            git.MustIDFromString("0000000000000000000000000000000000000000"),
		CommitMessage: "test",
	}
	dw.EntryName = "test.yml"
	dw.EntryDirectory = ".forgejo/workflows"
	dw.TriggerEvent = &jobparser.Event{
		Name: "pull_request",
	}
	detectedWorkflows := []*actions_module.DetectedWorkflow{dw}
	input := &NotifyInput{
		Repo:        repo,
		Doer:        doer,
		Event:       event,
		PullRequest: pr,
		Payload:     &api.PullRequestPayload{},
	}

	err := handleWorkflows(db.DefaultContext, detectedWorkflows, commit, input, "refs/head/main")
	require.NoError(t, err)
}

func TestActionsNotifier_OpenForkPullRequestEvent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})
	require.True(t, pr.IsFromFork())

	testActionsNotifierPullRequest(t, repo, pr, &actions_module.DetectedWorkflow{}, webhook_module.HookEventPullRequest)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)

	assert.Equal(t, webhook_module.HookEventPullRequest, runs[0].Event)
	assert.True(t, runs[0].IsForkPullRequest)
}

func TestActionsNotifier_ConcurrencyGroup(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content: []byte("{ on: pull_request, jobs: { j1: {} }}"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	firstRun := runs[0]

	assert.Equal(t, "refs/head/main_test.yml_pull_request__auto", firstRun.ConcurrencyGroup)
	assert.Equal(t, actions_model.CancelInProgress, firstRun.ConcurrencyType)
	assert.Equal(t, actions_model.StatusWaiting, firstRun.Status)

	// Also... check if CancelPreviousWithConcurrencyGroup is invoked from handleWorkflows by firing off a second
	// workflow and checking that the first one gets cancelled:

	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err = db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 2)

	firstRunIndex := slices.IndexFunc(runs, func(run *actions_model.ActionRun) bool { return run.ID == firstRun.ID })
	require.NotEqual(t, -1, firstRunIndex)
	firstRun = runs[firstRunIndex]

	assert.Equal(t, "refs/head/main_test.yml_pull_request__auto", firstRun.ConcurrencyGroup)
	assert.Equal(t, actions_model.CancelInProgress, firstRun.ConcurrencyType)
	assert.Equal(t, actions_model.StatusCancelled, firstRun.Status)
}

func TestActionsNotifier_PreExecutionErrorInvalidJobs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content: []byte("{ on: pull_request, jobs: 'hello, I am the jobs!' }"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	createdRun := runs[0]

	assert.Equal(t, actions_model.StatusFailure, createdRun.Status)
	assert.Empty(t, createdRun.PreExecutionError)
	assert.Equal(t, actions_model.ErrorCodeJobParsingError, createdRun.PreExecutionErrorCode)
	assert.Equal(t, []any{"model.ReadWorkflow: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `hello, ...` into map[string]*model.Job"}, createdRun.PreExecutionErrorDetails)
}

func TestActionsNotifier_PreExecutionEventDetectionError(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content:             []byte("{ on: nothing, jobs: { j1: {} }}"),
		EventDetectionError: errors.New("nothing is not a valid event"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	createdRun := runs[0]

	assert.Equal(t, actions_model.StatusFailure, createdRun.Status)
	assert.Empty(t, createdRun.PreExecutionError)
	assert.Equal(t, actions_model.ErrorCodeEventDetectionError, createdRun.PreExecutionErrorCode)
	assert.Equal(t, []any{"nothing is not a valid event"}, createdRun.PreExecutionErrorDetails)
}

func TestActionsNotifier_handleWorkflows_setRunTrustForPullRequest(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	// poster is not trusted implicitly
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	testActionsNotifierPullRequest(t, repo, pr, &actions_module.DetectedWorkflow{
		NeedApproval: true,
		Content:      []byte("on: pull_request\njobs: { job_a: {} }"),
	}, webhook_module.HookEventPullRequest)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)

	run := runs[0]
	assert.True(t, run.IsForkPullRequest)
	assert.Equal(t, pr.Issue.PosterID, run.PullRequestPosterID)
	assert.Equal(t, pr.ID, run.PullRequestID)
	assert.True(t, run.NeedApproval)
}

func TestActionsNotifier_DynamicMatrix(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content: []byte("{ on: pull_request, jobs: { j1: { strategy: { matrix: { dim1: \"${{ fromJSON(needs.other-job.outputs.some-output) }}\" } } } } }"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	run := runs[0]

	jobs, err := db.Find[actions_model.ActionRunJob](t.Context(), actions_model.FindRunJobOptions{RunID: run.ID})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	job := jobs[0]

	// With a matrix that contains ${{ needs ... }} references, the only requirement to work is that when the job is
	// first inserted it is tagged w/ incomplete_matrix
	assert.Contains(t, string(job.WorkflowPayload), "incomplete_matrix: true")
}

func TestActionsNotifier_RunsOnNeeds(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content: []byte("{ on: pull_request, jobs: { j1: { runs-on: \"${{ needs.other-job.outputs.some-output }}\" } } }"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	run := runs[0]

	jobs, err := db.Find[actions_model.ActionRunJob](t.Context(), actions_model.FindRunJobOptions{RunID: run.ID})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	job := jobs[0]

	// With a runs-on that contains ${{ needs ... }} references, the only requirement to work is that when the job is
	// first inserted it is tagged w/ incomplete_runs_on.
	assert.Contains(t, string(job.WorkflowPayload), "incomplete_runs_on: true")
}

func TestActionsNotifier_WorkflowDetection(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content: []byte("{ on: pull_request, jobs: { j1: {} }}"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	run := runs[0]

	jobs, err := db.Find[actions_model.ActionRunJob](t.Context(), actions_model.FindRunJobOptions{RunID: run.ID})
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	assert.Equal(t, ".forgejo/workflows", runs[0].WorkflowDirectory)
	assert.Equal(t, "test.yml", runs[0].WorkflowID)
}

// Verifies that the notifier_helper's `handleWorkflows` provides the local & remote reusable workflow expansion
// routines to the jobparser, and that data flows into them accurately.
func TestActionsNotifier_ExpandReusableWorkflow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	var localReusableCalled []string
	var localReusableCalledGitCommit []*git.Commit
	defer test.MockVariableValue(&expandLocalReusableWorkflows,
		func(commit *git.Commit) jobparser.LocalWorkflowFetcher {
			return func(job *jobparser.Job, path string) ([]byte, error) {
				localReusableCalledGitCommit = append(localReusableCalledGitCommit, commit)
				localReusableCalled = append(localReusableCalled, path)
				return []byte("{ on: pull_request, jobs: { j1: { runs-on: debian-latest } } }"), nil
			}
		})()
	remoteReusableCalled := []*model.NonLocalReusableWorkflowReference{}
	defer test.MockVariableValue(&expandInstanceReusableWorkflows,
		func(ctx context.Context) jobparser.InstanceWorkflowFetcher {
			return func(job *jobparser.Job, ref *model.NonLocalReusableWorkflowReference) ([]byte, error) {
				remoteReusableCalled = append(remoteReusableCalled, ref)
				return []byte("{ on: pull_request, jobs: { j1: { runs-on: debian-latest } } }"), nil
			}
		})()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content: []byte("{ on: pull_request, jobs: { j1: { uses: \"./.forgejo/workflows/reusable-path.yml\" }, j2: { uses: \"some-org/some-repo/.forgejo/workflows/reusable-path.yml@main\" }} }"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	run := runs[0]
	assert.EqualValues(t, 0, run.PreExecutionErrorCode, "pre execution error details: %#v", run.PreExecutionErrorDetails)

	require.Len(t, localReusableCalled, 1, "localReusableCalled")
	require.Len(t, localReusableCalledGitCommit, 1, "localReusableCalledGitCommit")
	require.Len(t, remoteReusableCalled, 1, "remoteReusableCalled")

	assert.Equal(t, "./.forgejo/workflows/reusable-path.yml", localReusableCalled[0])
	assert.Equal(t, "test", localReusableCalledGitCommit[0].CommitMessage)
	assert.Equal(t, &model.NonLocalReusableWorkflowReference{
		Org:         "some-org",
		Repo:        "some-repo",
		Filename:    "reusable-path.yml",
		Ref:         "main",
		GitPlatform: "forgejo",
	}, remoteReusableCalled[0])
}

func TestActionsNotifier_PermissionsWarning(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 3})

	dw := &actions_module.DetectedWorkflow{
		Content: []byte("{ on: pull_request, permissions: { contents: read }, jobs: { j1: { steps: [] } } }"),
	}
	testActionsNotifierPullRequest(t, repo, pr, dw, webhook_module.HookEventPullRequestSync)

	runs, err := db.Find[actions_model.ActionRun](db.DefaultContext, actions_model.FindRunOptions{
		RepoID: repo.ID,
	})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	run := runs[0]
	assert.EqualValues(t, 0, run.PreExecutionErrorCode, "pre execution error details: %#v", run.PreExecutionErrorDetails)

	assert.Equal(t, []actions_model.PreExecutionWarning{1}, run.PreExecutionWarningCodes)
	assert.Equal(t, [][]any{{"j1", "https://forgejo.org/docs/latest/user/authorized-integrations/"}}, run.PreExecutionWarningDetails)
}
