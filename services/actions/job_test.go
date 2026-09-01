// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/timeutil"
	notify_service "forgejo.org/services/notify"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDeleteJobsOfRun(t *testing.T) {
	t.Run("Deletes completed job", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteJobsOfRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34901})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 47301, RunID: run.ID})
		unittest.AssertCount(t, &actions_model.ActionTask{JobID: job.ID}, 1)

		require.NoError(t, deleteJobsOfRun(t.Context(), run.ID))

		unittest.AssertNotExistsBean(t, &actions_model.ActionRunJob{ID: job.ID})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 47302})
		unittest.AssertCount(t, &actions_model.ActionTask{JobID: job.ID}, 0)
	})

	t.Run("Error if job has not completed", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteJobsOfRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34902})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 47302, RunID: run.ID})
		unittest.AssertCount(t, &actions_model.ActionTask{JobID: job.ID}, 1)

		err := deleteJobsOfRun(t.Context(), run.ID)
		require.ErrorContains(t, err, "unable to delete job 47302 because it has not completed yet")

		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		unittest.AssertCount(t, &actions_model.ActionTask{JobID: job.ID}, 1)
	})
}

func TestConvertSingleWorkflowToJobs(t *testing.T) {
	t.Run("Incomplete matrix", func(t *testing.T) {
		runDoesNotNeedApproval := &actions_model.ActionRun{
			RepoID:              int64(10),
			PullRequestID:       int64(2),
			PullRequestPosterID: int64(4),
		}

		workflowRaw := []byte(`
jobs:
  job2:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        dim1: "${{ fromJSON(needs.other-job.outputs.some-output) }}"
    steps:
      - run: true
`)
		workflows, err := jobparser.Parse(workflowRaw, false, jobparser.WithJobOutputs(map[string]map[string]string{}))
		require.NoError(t, err)
		require.True(t, workflows[0].IncompleteMatrix) // must be set for this test scenario to be valid

		jobs, err := convertSingleWorkflowToJobs(runDoesNotNeedApproval, workflows)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		// Expect job with an incomplete matrix to be StatusBlocked:
		assert.Equal(t, actions_model.StatusBlocked, jobs[0].Status)
	})

	t.Run("Incomplete runs-on", func(t *testing.T) {
		runDoesNotNeedApproval := &actions_model.ActionRun{
			RepoID:              int64(10),
			PullRequestID:       int64(2),
			PullRequestPosterID: int64(4),
		}

		workflowRaw := []byte(`
jobs:
  job2:
    runs-on: ${{ needs.other-job.outputs.some-output }}
    steps:
      - run: true
`)
		workflows, err := jobparser.Parse(workflowRaw, false, jobparser.WithJobOutputs(map[string]map[string]string{}), jobparser.SupportIncompleteRunsOn())
		require.NoError(t, err)
		require.True(t, workflows[0].IncompleteRunsOn) // must be set for this test scenario to be valid

		jobs, err := convertSingleWorkflowToJobs(runDoesNotNeedApproval, workflows)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		// Expect job with an incomplete runs-on to be StatusBlocked:
		assert.Equal(t, actions_model.StatusBlocked, jobs[0].Status)
	})

	t.Run("Incomplete with", func(t *testing.T) {
		runDoesNotNeedApproval := &actions_model.ActionRun{
			RepoID:              int64(10),
			PullRequestID:       int64(2),
			PullRequestPosterID: int64(4),
		}

		workflowRaw := []byte(`
jobs:
  outer-job:
    with:
      some_input: ${{ needs.other-job.outputs.some-output }}
    uses: ./.forgejo/workflows/reusable.yml
`)
		workflows, err := jobparser.Parse(workflowRaw, false,
			jobparser.WithJobOutputs(map[string]map[string]string{}),
			jobparser.ExpandLocalReusableWorkflows(func(job *jobparser.Job, path string) ([]byte, error) {
				return []byte(`
on:
  workflow_call:
    inputs:
      some_input:
        type: string
jobs:
  inner-job:
    runs-on: debian
    steps: []
`), nil
			}))
		require.NoError(t, err)
		require.True(t, workflows[0].IncompleteWith) // must be set for this test scenario to be valid

		jobs, err := convertSingleWorkflowToJobs(runDoesNotNeedApproval, workflows)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		// Expect job with an incomplete with to be StatusBlocked:
		assert.Equal(t, actions_model.StatusBlocked, jobs[0].Status)
	})
}

func TestConvertSingleWorkflowToJobs_FindOuterWorkflowCall(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	run := &actions_model.ActionRun{
		RepoID:              int64(10),
		PullRequestID:       int64(2),
		PullRequestPosterID: int64(4),
	}

	workflowRaw := []byte(`
jobs:
  outer-job:
    uses: ./.forgejo/workflows/reusable.yml
`)
	workflows, err := jobparser.Parse(workflowRaw, false,
		jobparser.WithJobOutputs(map[string]map[string]string{}),
		jobparser.ExpandLocalReusableWorkflows(func(job *jobparser.Job, path string) ([]byte, error) {
			return []byte(`
on:
  workflow_call:
jobs:
  inner-job-1:
    runs-on: debian
    steps: []
  inner-job-2:
    runs-on: debian
    steps: []
`), nil
		}))
	require.NoError(t, err)

	jobs, err := convertSingleWorkflowToJobs(run, workflows)
	require.NoError(t, err)
	require.Len(t, jobs, 3)

	require.NoError(t, actions_model.InsertRunWithoutNotification(t.Context(), run, jobs))

	for _, j := range jobs {
		t.Run(j.Name, func(t *testing.T) {
			_, err := j.DecodeWorkflowPayload()
			require.NoError(t, err)
			outer, err := run.FindOuterWorkflowCall(t.Context(), j)
			if j.Name == "outer-job" {
				require.ErrorContains(t, err, "invalid state for FindOuterWorkflowCall")
			} else {
				require.NoError(t, err)
				require.NotNil(t, outer)
				assert.Equal(t, "outer-job", outer.Name)
			}
		})
	}
}

func TestInitiateNextJobAttempt(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	notifier := notify_service.NewMockNotifier(t)
	notifier.On("Run").Return().Maybe()
	notifier.On("NewWorkflowJobAttempt", mock.Anything, mock.Anything).Return(nil)

	notify_service.RegisterNotifier(notifier)
	defer notify_service.UnregisterNotifier(notifier)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	testWorkflows := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 62, OwnerID: user2.ID})

	run := &actions_model.ActionRun{
		ID:        101802,
		RepoID:    testWorkflows.ID,
		OwnerID:   user2.ID,
		CommitSHA: "cea0212242e69ad973e46a36a6af3d3999bb2989",
		Status:    actions_model.StatusSuccess,
	}
	job := &actions_model.ActionRunJob{
		ID:        252550,
		RunID:     run.ID,
		RepoID:    testWorkflows.ID,
		OwnerID:   user2.ID,
		CommitSHA: "cea0212242e69ad973e46a36a6af3d3999bb2989",
		Started:   timeutil.TimeStampNow().Add(-10),
		Stopped:   timeutil.TimeStampNow(),
		TaskID:    537550,
		Status:    actions_model.StatusSuccess,
		Attempt:   1,
		Handle:    "0acf574b-22cf-4c27-a3b0-775dc58018c1",
	}

	unittest.AssertSuccessfulInsert(t, run, job)

	require.NoError(t, InitiateNextJobAttempt(t.Context(), job, actions_model.StatusBlocked))

	job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})

	assert.EqualValues(t, 2, job.Attempt)
	assert.Zero(t, job.Started)
	assert.Zero(t, job.Stopped)
	assert.Zero(t, job.TaskID)
	assert.Len(t, job.Handle, 36)
	assert.NotEqual(t, "0acf574b-22cf-4c27-a3b0-775dc58018c1", job.Handle)
	assert.Equal(t, actions_model.StatusBlocked, job.Status)

	notifier.AssertNumberOfCalls(t, "NewWorkflowJobAttempt", 1)
	notifier.AssertCalled(
		t, "NewWorkflowJobAttempt", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == 252550 && job.Status == actions_model.StatusBlocked &&
				job.Attempt == 2 && job.Run != nil
		}),
	)
}

func TestPropagateJobStatus(t *testing.T) {
	t.Run("No status change", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestPropagateJobStatus")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 252551})

		require.NoError(t, PropagateJobStatus(t.Context(), job.ID, job.Status))
	})

	t.Run("Status change", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestPropagateJobStatus")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowJobStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		require.NoError(t, PropagateJobStatus(t.Context(), 252551, actions_model.StatusBlocked))

		notifier.AssertNumberOfCalls(t, "WorkflowJobStatusChanged", 1)
		notifier.AssertCalled(
			t, "WorkflowJobStatusChanged", mock.Anything,
			mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
				return job.ID == 252551 && job.Status == actions_model.StatusWaiting && job.Run != nil
			}),
			actions_model.StatusBlocked,
		)
	})

	t.Run("Job completion", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestPropagateJobStatus")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 252551})

		job.Status = actions_model.StatusCancelled
		_, err := actions_model.UpdateRunJobWithoutNotification(t.Context(), job, nil)
		require.NoError(t, err)

		require.NoError(t, PropagateJobStatus(t.Context(), 252551, actions_model.StatusWaiting))

		notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 1)
		notifier.AssertCalled(
			t, "WorkflowJobCompleted", mock.Anything,
			mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
				return job.ID == 252551 && job.Status == actions_model.StatusCancelled && job.Run != nil
			}),
			actions_model.StatusWaiting,
		)
	})

	t.Run("Incomplete job", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestPropagateJobStatus")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 252551})

		job.WorkflowPayload = []byte(`
"on":
    push:
    workflow_dispatch:
jobs:
    test:
        name: test (incomplete matrix)
        runs-on: docker
        steps:
            - uses: https://code.forgejo.org/actions/setup-node@v4
              with:
                node-version: ${{ matrix.node }}
        strategy:
            matrix:
                variant: ${{ fromJSON(needs.matrix-generator.outputs.variants) }}
                node: ${{ fromJSON(needs.matrix-generator.outputs.nodes) }}
enable-openid-connect: true
incomplete_matrix: true
incomplete_matrix_needs:
    job: matrix-generator
    output: variants
`)
		job.Status = actions_model.StatusBlocked
		_, err := actions_model.UpdateRunJobWithoutNotification(t.Context(), job, nil)
		require.NoError(t, err)

		require.NoError(t, PropagateJobStatus(t.Context(), job.ID, actions_model.StatusWaiting))
	})
}

func TestPropagateNextJobAttempt(t *testing.T) {
	t.Run("New attempt", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestPropagateJobStatus")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("NewWorkflowJobAttempt", mock.Anything, mock.Anything).Return(nil)

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		require.NoError(t, PropagateNextJobAttempt(t.Context(), 252551))

		notifier.AssertNumberOfCalls(t, "NewWorkflowJobAttempt", 1)
		notifier.AssertCalled(
			t, "NewWorkflowJobAttempt", mock.Anything,
			mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
				return job.ID == 252551 && job.Status == actions_model.StatusWaiting && job.Run != nil
			}),
		)
	})

	t.Run("Incomplete job", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestPropagateJobStatus")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 252551})

		job.WorkflowPayload = []byte(`
"on":
    push:
    workflow_dispatch:
jobs:
    test:
        name: test (incomplete matrix)
        runs-on: docker
        steps:
            - uses: https://code.forgejo.org/actions/setup-node@v4
              with:
                node-version: ${{ matrix.node }}
        strategy:
            matrix:
                variant: ${{ fromJSON(needs.matrix-generator.outputs.variants) }}
                node: ${{ fromJSON(needs.matrix-generator.outputs.nodes) }}
enable-openid-connect: true
incomplete_matrix: true
incomplete_matrix_needs:
    job: matrix-generator
    output: variants
`)
		job.Status = actions_model.StatusBlocked
		_, err := actions_model.UpdateRunJobWithoutNotification(t.Context(), job, nil)
		require.NoError(t, err)

		require.NoError(t, PropagateNextJobAttempt(t.Context(), job.ID))
	})
}
