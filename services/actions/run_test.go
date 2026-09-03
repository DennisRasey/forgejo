// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"testing"
	"time"

	actions_model "forgejo.org/models/actions"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/timeutil"
	notify_service "forgejo.org/services/notify"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestActions_CancelOrApproveRun(t *testing.T) {
	t.Run("run, job and task Running changes to run, job and task Cancelled", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestActions_CancelOrApproveRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return()
		notifier.On("WorkflowRunCompleted", mock.Anything, mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		taskID := int64(711900)
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: taskID})
		require.Equal(t, actions_model.StatusRunning.String(), task.Status.String())
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
		require.Equal(t, actions_model.StatusRunning.String(), job.Status.String())
		require.Zero(t, job.Stopped)
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		require.Equal(t, actions_model.StatusRunning.String(), run.Status.String())

		require.NoError(t, CancelRun(t.Context(), run))

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
		assert.Equal(t, actions_model.StatusCancelled.String(), job.Status.String())
		assert.NotZero(t, job.Stopped)
		task = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: taskID})
		require.Equal(t, actions_model.StatusCancelled.String(), task.Status.String())

		notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 1)
		notifier.AssertNumberOfCalls(t, "WorkflowRunCompleted", 1)

		notifier.AssertCalled(
			t, "WorkflowJobCompleted", mock.Anything,
			mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
				return job.ID == task.JobID && job.Status == actions_model.StatusCancelled
			}),
			actions_model.StatusRunning,
		)
		notifier.AssertCalled(
			t, "WorkflowRunCompleted", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == job.RunID && run.Status == actions_model.StatusCancelled
			}),
			actions_model.StatusRunning,
		)
	})

	t.Run("run Running, job and task Success changes to run Cancelled", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestActions_CancelOrApproveRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		taskID := int64(710900)
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: taskID})
		require.Equal(t, actions_model.StatusSuccess.String(), task.Status.String())
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
		require.Equal(t, actions_model.StatusSuccess.String(), job.Status.String())
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		require.Equal(t, actions_model.StatusRunning.String(), run.Status.String())

		require.NoError(t, CancelRun(t.Context(), run))

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
		assert.Equal(t, actions_model.StatusSuccess, job.Status)
		task = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: taskID})
		require.Equal(t, actions_model.StatusSuccess, task.Status)
	})

	t.Run("run Waiting and job Blocked for Approval changes to run and job Cancelled", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestActions_CancelOrApproveRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		jobID := int64(10800)
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: jobID})
		require.Equal(t, actions_model.StatusBlocked.String(), job.Status.String())
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		require.Equal(t, actions_model.StatusBlocked.String(), run.Status.String())
		require.True(t, run.NeedApproval)

		require.NoError(t, CancelRun(t.Context(), run))

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		assert.Equal(t, actions_model.StatusCancelled.String(), run.Status.String())
		assert.False(t, run.NeedApproval)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: jobID})
		assert.Equal(t, actions_model.StatusCancelled, job.Status)
	})

	t.Run("run Waiting and job Blocked for Approval changes to job Waiting", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestActions_CancelOrApproveRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowJobStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return()
		notifier.On("WorkflowRunStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		jobID := int64(10800)
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: jobID})
		require.Equal(t, actions_model.StatusBlocked.String(), job.Status.String())
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		require.Equal(t, actions_model.StatusBlocked.String(), run.Status.String())
		require.True(t, run.NeedApproval)

		doerID := int64(30)
		require.NoError(t, ApproveRun(t.Context(), run, doerID))

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})
		assert.Equal(t, actions_model.StatusWaiting.String(), run.Status.String())
		assert.False(t, run.NeedApproval)
		assert.Equal(t, doerID, run.ApprovedBy)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: jobID})
		assert.Equal(t, actions_model.StatusWaiting, job.Status)

		notifier.AssertNumberOfCalls(t, "WorkflowJobStatusChanged", 1)
		notifier.AssertNumberOfCalls(t, "WorkflowRunStatusChanged", 1)

		notifier.AssertCalled(
			t, "WorkflowJobStatusChanged", mock.Anything,
			mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
				return job.ID == 10800 && job.Status == actions_model.StatusWaiting
			}),
			actions_model.StatusBlocked,
		)
		notifier.AssertCalled(
			t, "WorkflowRunStatusChanged", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == job.RunID && run.Status == actions_model.StatusWaiting
			}),
			actions_model.StatusBlocked,
		)
	})
}

func TestActions_consistencyCheckRun(t *testing.T) {
	tests := []struct {
		name                     string
		runID                    int64
		errContains              string
		consumed                 bool
		runJobNames              []string
		preExecutionError        actions_model.PreExecutionError
		preExecutionErrorDetails []any
	}{
		{
			name:  "consistent: not incomplete_matrix",
			runID: 900,
		},
		{
			name:  "consistent: incomplete_matrix all needs exist",
			runID: 901,
		},
		{
			name:                     "inconsistent: incomplete_matrix all needs exist",
			runID:                    902,
			preExecutionError:        actions_model.ErrorCodeIncompleteMatrixMissingJob,
			preExecutionErrorDetails: []any{"job_1", "oops-something-wrong-here", "define-matrix"},
		},
		{
			name:                     "inconsistent: static matrix missing dimension",
			runID:                    903,
			preExecutionError:        actions_model.ErrorCodeIncompleteRunsOnMissingMatrixDimension,
			preExecutionErrorDetails: []any{"job_1", "platform-oops-wrong-dimension"},
		},
		{
			name:  "consistent: matrix missing dimension but matrix is dynamic",
			runID: 904,
		},
		{
			name:                     "unknown job in needs",
			runID:                    905,
			preExecutionError:        actions_model.ErrorCodeUnknownJobInNeeds,
			preExecutionErrorDetails: []any{"job_2", "unknown, Job_1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer unittest.OverrideFixtures("services/actions/TestActions_consistencyCheckRun")()
			require.NoError(t, unittest.PrepareTestDatabase())

			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: tt.runID})

			err := consistencyCheckRun(t.Context(), run)
			require.NoError(t, err)

			run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: tt.runID})
			assert.Equal(t, tt.preExecutionError, run.PreExecutionErrorCode)
			assert.Equal(t, tt.preExecutionErrorDetails, run.PreExecutionErrorDetails)
		})
	}
}

func TestDeleteRun(t *testing.T) {
	t.Run("Removes run and its dependencies", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34901})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID})
		unittest.AssertCount(t, &actions_model.ActionArtifact{RunID: run.ID}, 2)

		require.NoError(t, DeleteRun(t.Context(), run.ID))

		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: run.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRunJob{ID: job.ID})
		unittest.AssertCount(t, &actions_model.ActionArtifact{
			RunID:  run.ID,
			Status: int64(actions_model.ArtifactStatusPendingDeletion),
		}, 2)
	})

	t.Run("Error if run not done", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteRun")()
		require.NoError(t, unittest.PrepareTestDatabase())

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34902})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID})
		unittest.AssertCount(t, &actions_model.ActionArtifact{RunID: run.ID}, 1)

		err := DeleteRun(t.Context(), run.ID)
		require.ErrorContains(t, err, "cannot delete run 34902 because it has not completed yet")

		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		unittest.AssertCount(t, &actions_model.ActionArtifact{
			RunID:  run.ID,
			Status: int64(actions_model.ArtifactStatusUploadConfirmed),
		}, 1)
	})
}

func TestPrioritizeRun(t *testing.T) {
	t.Run("Run prioritized", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		runOne := &actions_model.ActionRun{
			ID: 408911, Index: 1, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting,
			Priority: actions_model.DefaultRunPriority, Prioritize: false,
		}
		runTwo := &actions_model.ActionRun{
			ID: 408912, Index: 2, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting, Priority: 25,
		}
		unittest.AssertSuccessfulInsert(t, runOne, runTwo)

		err := PrioritizeRun(t.Context(), runOne)
		require.NoError(t, err)

		prioritizedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runOne.ID})
		assert.True(t, prioritizedRun.Prioritize)
		assert.Equal(t, actions_model.MaxRunPriority, prioritizedRun.Priority)

		// Verify that the priority of the unrelated run has been recalculated, too.
		waitingRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runTwo.ID})
		assert.False(t, waitingRun.Prioritize)
		assert.Equal(t, actions_model.DefaultRunPriority, waitingRun.Priority)
	})

	t.Run("Nothing happens if run already prioritized", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		runOne := &actions_model.ActionRun{
			ID: 408911, Index: 1, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting,
			Priority: actions_model.MaxRunPriority, Prioritize: true,
		}
		runTwo := &actions_model.ActionRun{
			ID: 408912, Index: 2, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting, Priority: 25,
		}
		unittest.AssertSuccessfulInsert(t, runOne, runTwo)

		err := PrioritizeRun(t.Context(), runOne)
		require.NoError(t, err)

		prioritizedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runOne.ID})
		assert.True(t, prioritizedRun.Prioritize)
		assert.Equal(t, actions_model.MaxRunPriority, prioritizedRun.Priority)

		// Verify that the priority of the unrelated run not been recalculated.
		waitingRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runTwo.ID})
		assert.False(t, waitingRun.Prioritize)
		assert.Equal(t, int8(25), waitingRun.Priority)
	})

	t.Run("Completed run can be prioritized", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		testRun := &actions_model.ActionRun{
			ID:         808441,
			Index:      1,
			RepoID:     62,
			OwnerID:    2,
			Status:     actions_model.StatusSuccess,
			Priority:   actions_model.DefaultRunPriority,
			Prioritize: false,
		}
		unittest.AssertSuccessfulInsert(t, testRun)

		err := PrioritizeRun(t.Context(), testRun)
		require.NoError(t, err)

		prioritizedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: testRun.ID})
		assert.True(t, prioritizedRun.Prioritize)
		assert.Equal(t, actions_model.DefaultRunPriority, prioritizedRun.Priority) // Unchanged because run completed.
	})

	t.Run("Error if run is nil", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		err := PrioritizeRun(t.Context(), nil)
		require.ErrorContains(t, err, "run is nil")
	})
}

func TestDeprioritizeRun(t *testing.T) {
	t.Run("Run deprioritized", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		runOne := &actions_model.ActionRun{
			ID: 408911, Index: 1, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting,
			Priority: actions_model.MaxRunPriority, Prioritize: true,
		}
		runTwo := &actions_model.ActionRun{
			ID: 408912, Index: 2, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting, Priority: 25,
		}
		unittest.AssertSuccessfulInsert(t, runOne, runTwo)

		err := DeprioritizeRun(t.Context(), runOne)
		require.NoError(t, err)

		deprioritizedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runOne.ID})
		assert.False(t, deprioritizedRun.Prioritize)
		assert.Equal(t, actions_model.DefaultRunPriority, deprioritizedRun.Priority)

		// Verify that the priority of the unrelated run has been recalculated, too.
		waitingRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runTwo.ID})
		assert.False(t, waitingRun.Prioritize)
		assert.Equal(t, actions_model.DefaultRunPriority, waitingRun.Priority)
	})

	t.Run("Nothing happens if run not prioritized", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		runOne := &actions_model.ActionRun{
			ID: 408911, Index: 1, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting,
			Priority: actions_model.DefaultRunPriority, Prioritize: false,
		}
		runTwo := &actions_model.ActionRun{
			ID: 408912, Index: 2, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting, Priority: 25,
		}
		unittest.AssertSuccessfulInsert(t, runOne, runTwo)

		err := DeprioritizeRun(t.Context(), runOne)
		require.NoError(t, err)

		deprioritizedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runOne.ID})
		assert.False(t, deprioritizedRun.Prioritize)
		assert.Equal(t, actions_model.DefaultRunPriority, deprioritizedRun.Priority)

		// Verify that the priority of the unrelated run has *not* been recalculated.
		waitingRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runTwo.ID})
		assert.False(t, waitingRun.Prioritize)
		assert.Equal(t, int8(25), waitingRun.Priority)
	})

	t.Run("Completed run can be deprioritized", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		testRun := &actions_model.ActionRun{
			ID:         535681,
			Index:      1,
			RepoID:     62,
			OwnerID:    2,
			Status:     actions_model.StatusSuccess,
			Priority:   actions_model.MaxRunPriority,
			Prioritize: true,
		}
		unittest.AssertSuccessfulInsert(t, testRun)

		err := DeprioritizeRun(t.Context(), testRun)
		require.NoError(t, err)

		deprioritizedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: testRun.ID})
		assert.False(t, deprioritizedRun.Prioritize)
		assert.Equal(t, actions_model.MaxRunPriority, deprioritizedRun.Priority) // Unchanged because run completed.
	})

	t.Run("Error if run is nil", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		err := DeprioritizeRun(t.Context(), nil)
		require.ErrorContains(t, err, "run is nil")
	})
}

func TestRecalculateRunPriorities(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	fixtures := []*actions_model.ActionRun{
		{ID: 535681, Index: 1, RepoID: 62, OwnerID: 2, Status: actions_model.StatusSuccess},
		{ID: 535682, Index: 2, RepoID: 62, OwnerID: 2, Status: actions_model.StatusRunning, Priority: actions_model.DefaultRunPriority, Prioritize: true},
		{ID: 535683, Index: 3, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting, Priority: actions_model.DefaultRunPriority, Prioritize: true},
		{ID: 535684, Index: 4, RepoID: 62, OwnerID: 2, Status: actions_model.StatusBlocked, Priority: actions_model.MaxRunPriority},
		{ID: 535685, Index: 1, RepoID: 1, OwnerID: 2, Status: actions_model.StatusBlocked, Priority: actions_model.DefaultRunPriority, Prioritize: true},
		{ID: 535686, Index: 5, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting},
	}
	unittest.AssertSuccessfulInsert(t, fixtures)

	err := recalculateRunPriorities(t.Context(), 62)
	require.NoError(t, err)

	runOne := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535681})
	assert.Equal(t, actions_model.DefaultRunPriority, runOne.Priority)
	assert.False(t, runOne.Prioritize)

	runTwo := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535682})
	assert.Equal(t, actions_model.DefaultRunPriority, runTwo.Priority) // Unchanged because already running.
	assert.True(t, runTwo.Prioritize)

	runThree := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535683})
	assert.Equal(t, actions_model.MaxRunPriority, runThree.Priority)
	assert.True(t, runThree.Prioritize)

	runFour := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535684})
	assert.Equal(t, actions_model.DefaultRunPriority, runFour.Priority)
	assert.False(t, runFour.Prioritize)

	runFive := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535685})
	assert.Equal(t, actions_model.DefaultRunPriority, runFive.Priority) // Unchanged because different repository.
	assert.True(t, runFive.Prioritize)

	runSix := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535686})
	assert.Equal(t, actions_model.DefaultRunPriority, runSix.Priority)
	assert.False(t, runSix.Prioritize)
}

func TestInitiateNextRunAttempt(t *testing.T) {
	fixtures := []*actions_model.ActionRun{
		{
			ID:               535681,
			Index:            1,
			RepoID:           62,
			OwnerID:          2,
			Status:           actions_model.StatusSuccess,
			Priority:         actions_model.MaxRunPriority,
			Prioritize:       true,
			Started:          1786976036,
			Stopped:          1786976040,
			PreviousDuration: 60 * time.Second,
		},
		{
			ID:      535682,
			Index:   2,
			RepoID:  62,
			OwnerID: 2,
			Status:  actions_model.StatusRunning,
		},
	}

	t.Run("Prepared if completed", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		unittest.AssertSuccessfulInsert(t, fixtures)

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("NewWorkflowRunAttempt", mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535681})

		err := InitiateNextRunAttempt(t.Context(), run)
		require.NoError(t, err)

		// Verify that run has been written to database.
		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535681})

		assert.Equal(t, time.Minute+4*time.Second, run.PreviousDuration)
		assert.Equal(t, actions_model.StatusWaiting, run.Status)
		assert.Zero(t, run.Started)
		assert.Zero(t, run.Stopped)
		assert.Equal(t, actions_model.DefaultRunPriority, run.Priority)
		assert.False(t, run.Prioritize)

		notifier.AssertNumberOfCalls(t, "NewWorkflowRunAttempt", 1)
		notifier.AssertCalled(
			t, "NewWorkflowRunAttempt", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == 535681 && run.Status == actions_model.StatusWaiting
			}),
		)
	})

	t.Run("Error if active", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		unittest.AssertSuccessfulInsert(t, fixtures)

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535682})

		err := InitiateNextRunAttempt(t.Context(), run)

		require.ErrorContains(t, err, "cannot prepare next attempt because run 535682 is active")
	})
}

func TestRefreshAndPropagateRunStatus(t *testing.T) {
	fixtures := []*actions_model.ActionRun{
		{ID: 535681, Index: 1, RepoID: 62, OwnerID: 2, Status: actions_model.StatusWaiting},
	}

	t.Run("No notification without change", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		unittest.AssertSuccessfulInsert(t, fixtures)

		job := &actions_model.ActionRunJob{
			ID:      748211,
			RunID:   535681,
			RepoID:  62,
			OwnerID: 2,
			Status:  actions_model.StatusWaiting,
		}

		unittest.AssertSuccessfulInsert(t, job)

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 535681})

		require.NoError(t, RefreshAndPropagateRunStatus(t.Context(), run.ID))

		assert.Equal(t, actions_model.StatusWaiting, run.Status)
	})

	t.Run("Status change notification", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		unittest.AssertSuccessfulInsert(t, fixtures)

		job := &actions_model.ActionRunJob{
			ID:      748211,
			RunID:   535681,
			RepoID:  62,
			OwnerID: 2,
			Status:  actions_model.StatusRunning,
		}

		unittest.AssertSuccessfulInsert(t, job)

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowRunStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		require.NoError(t, RefreshAndPropagateRunStatus(t.Context(), job.RunID))

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})

		assert.Equal(t, actions_model.StatusRunning, run.Status)

		notifier.AssertNumberOfCalls(t, "WorkflowRunStatusChanged", 1)
		notifier.AssertCalled(
			t, "WorkflowRunStatusChanged", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == 535681 && run.Status == actions_model.StatusRunning && run.Repo != nil
			}),
			actions_model.StatusWaiting,
		)
	})

	t.Run("Completed notification upon completion", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		unittest.AssertSuccessfulInsert(t, fixtures)

		job := &actions_model.ActionRunJob{
			ID:      748211,
			RunID:   535681,
			RepoID:  62,
			OwnerID: 2,
			Status:  actions_model.StatusSkipped,
		}

		unittest.AssertSuccessfulInsert(t, job)

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowRunCompleted", mock.Anything, mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		require.NoError(t, RefreshAndPropagateRunStatus(t.Context(), job.RunID))

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID})

		assert.Equal(t, actions_model.StatusSkipped, run.Status)

		notifier.AssertNumberOfCalls(t, "WorkflowRunCompleted", 1)
		notifier.AssertCalled(
			t, "WorkflowRunCompleted", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == job.RunID && run.Status == actions_model.StatusSkipped && run.Repo != nil
			}),
			actions_model.StatusWaiting,
		)
	})
}

func TestFailRunPreExecutionError(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	timeutil.MockSet(time.Date(2026, 8, 25, 13, 36, 12, 0, time.UTC))
	defer timeutil.MockUnset()

	notifier := notify_service.NewMockNotifier(t)
	notifier.On("Run").Return().Maybe()
	notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return()
	notifier.On("WorkflowRunCompleted", mock.Anything, mock.Anything, mock.Anything).Return()

	notify_service.RegisterNotifier(notifier)
	defer notify_service.UnregisterNotifier(notifier)

	run := &actions_model.ActionRun{
		ID:      541161,
		Title:   "Test run",
		OwnerID: 2,
		RepoID:  62,
		Status:  actions_model.StatusWaiting,
	}
	unittest.AssertSuccessfulInsert(t, run)

	job := &actions_model.ActionRunJob{
		ID:      880758,
		RunID:   run.ID,
		OwnerID: 2,
		RepoID:  62,
		Status:  actions_model.StatusWaiting,
	}
	unittest.AssertSuccessfulInsert(t, job)

	require.NoError(t, FailRunPreExecutionError(t.Context(), run, actions_model.ErrorCodeJobParsingError, []any{123}))

	run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})

	assert.Equal(t, actions_model.StatusFailure, run.Status)
	assert.Equal(t, actions_model.ErrorCodeJobParsingError, run.PreExecutionErrorCode)
	assert.Equal(t, []any{float64(123)}, run.PreExecutionErrorDetails)

	job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})

	assert.Equal(t, actions_model.StatusFailure, job.Status)
	assert.Equal(t, timeutil.TimeStamp(1787664972), job.Stopped)

	notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 1)
	notifier.AssertNumberOfCalls(t, "WorkflowRunCompleted", 1)

	notifier.AssertCalled(
		t, "WorkflowJobCompleted", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == 880758 && job.Status == actions_model.StatusFailure
		}),
		actions_model.StatusWaiting,
	)
	notifier.AssertCalled(
		t, "WorkflowRunCompleted", mock.Anything,
		mock.MatchedBy(func(run *actions_model.ActionRun) bool {
			return run.ID == job.RunID && run.Status == actions_model.StatusFailure
		}),
		actions_model.StatusWaiting,
	)
}

func TestInsertRun(t *testing.T) {
	t.Run("Triggers notifications", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("NewWorkflowJobAttempt", mock.Anything, mock.Anything).Return()
		notifier.On("NewWorkflowRunAttempt", mock.Anything, mock.Anything).Return()
		notifier.On("WorkflowRunStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user.ID})

		workflow := []byte(`
on:
  push:
jobs:
  build:
    runs-on: debian
    steps:
      - run: echo OK
`)

		run := &actions_model.ActionRun{
			ID:      541161,
			Title:   "Test run",
			OwnerID: user.ID,
			RepoID:  repo.ID,
			Status:  actions_model.StatusBlocked,
		}

		sw, err := jobparser.Parse(workflow, false)
		require.NoError(t, err)

		require.NoError(t, InsertRun(t.Context(), run, sw))

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})

		assert.Equal(t, actions_model.StatusWaiting, run.Status)

		notifier.AssertNumberOfCalls(t, "NewWorkflowJobAttempt", 1)
		notifier.AssertNumberOfCalls(t, "NewWorkflowRunAttempt", 1)
		notifier.AssertNumberOfCalls(t, "WorkflowRunStatusChanged", 1)
		notifier.AssertCalled(
			t, "NewWorkflowJobAttempt", mock.Anything,
			mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
				return job.RunID == 541161 && job.Status == actions_model.StatusWaiting && job.Run != nil
			}),
		)
		notifier.AssertCalled(
			t, "NewWorkflowRunAttempt", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == 541161 && run.Status == actions_model.StatusBlocked
			}),
		)
		notifier.AssertCalled(
			t, "WorkflowRunStatusChanged", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == 541161 && run.Status == actions_model.StatusWaiting
			}),
			actions_model.StatusBlocked,
		)
	})
}
