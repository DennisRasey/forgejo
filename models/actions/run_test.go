// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/cache"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"

	gouuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetConcurrencyGroup(t *testing.T) {
	run := ActionRun{}
	run.SetConcurrencyGroup("abc123")
	assert.Equal(t, "abc123", run.ConcurrencyGroup)
	run.SetConcurrencyGroup("ABC123") // case should collapse in SetConcurrencyGroup
	assert.Equal(t, "abc123", run.ConcurrencyGroup)
}

func TestSetDefaultConcurrencyGroup(t *testing.T) {
	run := ActionRun{
		Ref:          "refs/heads/main",
		WorkflowID:   "testing",
		TriggerEvent: "pull_request",
	}
	run.SetDefaultConcurrencyGroup()
	assert.Equal(t, "refs/heads/main_testing_pull_request__auto", run.ConcurrencyGroup)
	run = ActionRun{
		Ref:          "refs/heads/main",
		WorkflowID:   "TESTING", // case should collapse in SetDefaultConcurrencyGroup
		TriggerEvent: "pull_request",
	}
	run.SetDefaultConcurrencyGroup()
	assert.Equal(t, "refs/heads/main_testing_pull_request__auto", run.ConcurrencyGroup)
}

func TestGetWorkflowPath(t *testing.T) {
	run := ActionRun{
		WorkflowID:        "ci.yml",
		WorkflowDirectory: ".some/path/to/workflows",
	}
	assert.Equal(t, ".some/path/to/workflows/ci.yml", run.WorkflowPath())
}

func TestGetCommitLink(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	run := ActionRun{
		Repo:      repo,
		CommitSHA: "a356d1f1f82945a039cd16d4ce0137bd55284e77",
	}
	assert.Equal(t, "/sub/user2/repo1/commit/a356d1f1f82945a039cd16d4ce0137bd55284e77", run.CommitLink())
}

func TestIsScheduledRun(t *testing.T) {
	scheduledRun := ActionRun{
		CommitSHA:    "a356d1f1f82945a039cd16d4ce0137bd55284e77",
		TriggerEvent: "schedule",
	}
	pushRun := ActionRun{
		CommitSHA:    "8f9b5c6ab342eb11d7422deecef7195b18058b90",
		TriggerEvent: "push",
	}

	assert.True(t, scheduledRun.IsScheduledRun())
	assert.False(t, pushRun.IsScheduledRun())
}

func TestIsManualRun(t *testing.T) {
	manualRunRun := ActionRun{
		CommitSHA:    "a356d1f1f82945a039cd16d4ce0137bd55284e77",
		TriggerEvent: "workflow_dispatch",
	}
	pushRun := ActionRun{
		CommitSHA:    "8f9b5c6ab342eb11d7422deecef7195b18058b90",
		TriggerEvent: "push",
	}

	assert.True(t, manualRunRun.IsDispatchedRun())
	assert.False(t, pushRun.IsDispatchedRun())
}

func TestActionRun_IsValid(t *testing.T) {
	testCases := []struct {
		name    string
		run     ActionRun
		isValid bool
	}{
		{
			name:    "valid run",
			run:     ActionRun{},
			isValid: true,
		},
		{
			name:    "with pre-execution error",
			run:     ActionRun{PreExecutionErrorCode: ErrorCodeIncompleteRunsOnMissingOutput},
			isValid: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.isValid, testCase.run.IsValid())
		})
	}
}

func TestActionRun_CanBeRerun(t *testing.T) {
	testCases := []struct {
		name       string
		run        ActionRun
		canBeRerun bool
	}{
		{
			name:       "run with unknown status",
			run:        ActionRun{Status: StatusUnknown},
			canBeRerun: false,
		},
		{
			name:       "successful run",
			run:        ActionRun{Status: StatusSuccess},
			canBeRerun: true,
		},
		{
			name:       "failed run",
			run:        ActionRun{Status: StatusFailure},
			canBeRerun: true,
		},
		{
			name:       "cancelled run",
			run:        ActionRun{Status: StatusCancelled},
			canBeRerun: true,
		},
		{
			name:       "skipped run",
			run:        ActionRun{Status: StatusSkipped},
			canBeRerun: true,
		},
		{
			name:       "waiting run",
			run:        ActionRun{Status: StatusWaiting},
			canBeRerun: false,
		},
		{
			name:       "blocked run",
			run:        ActionRun{Status: StatusBlocked},
			canBeRerun: false,
		},
		{
			name:       "with pre-execution error",
			run:        ActionRun{PreExecutionErrorCode: ErrorCodeIncompleteRunsOnMissingOutput, Status: StatusSuccess},
			canBeRerun: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.canBeRerun, testCase.run.CanBeRerun())
		})
	}
}

func TestRepoNumOpenActions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	err := cache.Init()
	require.NoError(t, err)

	t.Run("Repo 1", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		clearRepoRunCountCache(t.Context(), repo.ID)
		assert.Equal(t, 0, RepoNumOpenActions(t.Context(), repo.ID))
	})

	t.Run("Repo 4", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		clearRepoRunCountCache(t.Context(), repo.ID)
		assert.Equal(t, 0, RepoNumOpenActions(t.Context(), repo.ID))
	})

	t.Run("Repo 63", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 63})
		clearRepoRunCountCache(t.Context(), repo.ID)
		assert.Equal(t, 1, RepoNumOpenActions(t.Context(), repo.ID))
	})

	t.Run("Cache Invalidation", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 63})
		assert.Equal(t, 1, RepoNumOpenActions(t.Context(), repo.ID))

		err = db.DeleteBeans(t.Context(), &ActionRun{RepoID: repo.ID})
		require.NoError(t, err)

		// Even though we've deleted ActionRun, expecting that the number of open runs is still 1 (cached)
		assert.Equal(t, 1, RepoNumOpenActions(t.Context(), repo.ID))

		// Now that we clear the cache, computation should be performed
		clearRepoRunCountCache(t.Context(), repo.ID)
		assert.Equal(t, 0, RepoNumOpenActions(t.Context(), repo.ID))
	})
}

func TestActionRun_GetRunsNotDoneByRepoIDAndPullRequestPosterID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repoID := int64(10)
	pullRequestID := int64(3)
	pullRequestPosterID := int64(30)

	runDone := &ActionRun{
		RepoID:              repoID,
		PullRequestID:       pullRequestID,
		PullRequestPosterID: pullRequestPosterID,
		Status:              StatusSuccess,
	}
	require.NoError(t, InsertRunWithoutNotification(t.Context(), runDone, nil))

	unrelatedUser := int64(5)
	runNotByPoster := &ActionRun{
		RepoID:              repoID,
		PullRequestID:       pullRequestID,
		PullRequestPosterID: unrelatedUser,
		Status:              StatusRunning,
	}
	require.NoError(t, InsertRunWithoutNotification(t.Context(), runNotByPoster, nil))

	unrelatedRepository := int64(6)
	runNotInTheSameRepository := &ActionRun{
		RepoID:              unrelatedRepository,
		PullRequestID:       pullRequestID,
		PullRequestPosterID: pullRequestPosterID,
		Status:              StatusSuccess,
	}
	require.NoError(t, InsertRunWithoutNotification(t.Context(), runNotInTheSameRepository, nil))

	for _, status := range []Status{StatusUnknown, StatusWaiting, StatusRunning} {
		t.Run(fmt.Sprintf("%s", status), func(t *testing.T) {
			runNotDone := &ActionRun{
				RepoID:              repoID,
				PullRequestID:       pullRequestID,
				Status:              status,
				PullRequestPosterID: pullRequestPosterID,
			}
			require.NoError(t, InsertRunWithoutNotification(t.Context(), runNotDone, nil))
			runs, err := GetRunsNotDoneByRepoIDAndPullRequestPosterID(t.Context(), repoID, pullRequestPosterID)
			require.NoError(t, err)
			require.Len(t, runs, 1)
			run := runs[0]
			assert.Equal(t, runNotDone.ID, run.ID)
			assert.Equal(t, runNotDone.Status, run.Status)
			unittest.AssertSuccessfulDelete(t, run)
		})
	}
}

func TestActionRun_NeedApproval(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pullRequestPosterID := int64(4)
	repoID := int64(10)
	pullRequestID := int64(2)
	runDoesNotNeedApproval := &ActionRun{
		RepoID:              repoID,
		PullRequestID:       pullRequestID,
		PullRequestPosterID: pullRequestPosterID,
	}
	require.NoError(t, InsertRunWithoutNotification(t.Context(), runDoesNotNeedApproval, nil))
	unrelatedRepository := int64(6)
	runNotInTheSameRepository := &ActionRun{
		RepoID:              unrelatedRepository,
		PullRequestID:       pullRequestID,
		PullRequestPosterID: pullRequestPosterID,
		NeedApproval:        true,
	}
	require.NoError(t, InsertRunWithoutNotification(t.Context(), runNotInTheSameRepository, nil))
	unrelatedPullRequest := int64(3)
	runNotInTheSamePullRequest := &ActionRun{
		RepoID:              repoID,
		PullRequestID:       unrelatedPullRequest,
		PullRequestPosterID: pullRequestPosterID,
		NeedApproval:        true,
	}
	require.NoError(t, InsertRunWithoutNotification(t.Context(), runNotInTheSamePullRequest, nil))

	t.Run("HasRunThatNeedApproval is false", func(t *testing.T) {
		has, err := HasRunThatNeedApproval(t.Context(), repoID, pullRequestID)
		require.NoError(t, err)
		assert.False(t, has)
	})

	runNeedApproval := &ActionRun{
		RepoID:              repoID,
		PullRequestID:       pullRequestID,
		PullRequestPosterID: pullRequestPosterID,
		NeedApproval:        true,
	}
	require.NoError(t, InsertRunWithoutNotification(t.Context(), runNeedApproval, nil))

	t.Run("HasRunThatNeedApproval is true", func(t *testing.T) {
		has, err := HasRunThatNeedApproval(t.Context(), repoID, pullRequestID)
		require.NoError(t, err)
		assert.True(t, has)
	})

	assertApprovalEqual := func(t *testing.T, expected, actual *ActionRun) {
		t.Helper()
		assert.Equal(t, expected.RepoID, actual.RepoID)
		assert.Equal(t, expected.PullRequestID, actual.PullRequestID)
		assert.Equal(t, expected.PullRequestPosterID, actual.PullRequestPosterID)
		assert.Equal(t, expected.NeedApproval, actual.NeedApproval)
	}

	t.Run("GetRunsThatNeedApproval", func(t *testing.T) {
		runs, err := GetRunsThatNeedApprovalByRepoIDAndPullRequestID(t.Context(), repoID, pullRequestID)
		require.NoError(t, err)
		require.Len(t, runs, 1)
		assertApprovalEqual(t, runNeedApproval, runs[0])
	})
}

func TestInsertRunJobs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	pullRequestPosterID := int64(4)
	repoID := int64(10)
	pullRequestID := int64(2)
	actionRun := &ActionRun{
		RepoID:              repoID,
		PullRequestID:       pullRequestID,
		PullRequestPosterID: pullRequestPosterID,
		CommitSHA:           "1421f75bc5474c69fdb1dc176bcb96d381f935dd",
	}

	unittest.AssertSuccessfulInsert(t, actionRun)

	jobs := []*ActionRunJob{
		{
			RepoID:    actionRun.RepoID,
			CommitSHA: actionRun.CommitSHA,
			JobID:     "build",
			Name:      "build",
			RunsOn:    []string{"fedora"},
			Status:    StatusWaiting,
		},
		{
			RepoID:    actionRun.RepoID,
			CommitSHA: actionRun.CommitSHA,
			JobID:     "test",
			Name:      "test",
			RunsOn:    []string{"debian"},
			Status:    StatusWaiting,
			Handle:    gouuid.New().String(),
		},
	}

	require.NoError(t, InsertRunJobs(t.Context(), actionRun, jobs))

	assert.NotZero(t, jobs[0].ID)
	assert.NotZero(t, jobs[1].ID)

	insertedJobs, err := db.Find[ActionRunJob](t.Context(), FindRunJobOptions{RunID: actionRun.ID})
	require.NoError(t, err)
	require.Len(t, insertedJobs, 2)

	assert.Equal(t, actionRun.ID, insertedJobs[0].RunID)
	assert.Equal(t, actionRun.RepoID, insertedJobs[0].RepoID)
	assert.Equal(t, actionRun.OwnerID, insertedJobs[0].OwnerID)
	assert.Equal(t, actionRun.CommitSHA, insertedJobs[0].CommitSHA)
	assert.Equal(t, actionRun.IsForkPullRequest, insertedJobs[0].IsForkPullRequest)
	assert.Equal(t, "build", insertedJobs[0].Name)
	assert.Equal(t, JobIdentifier("build"), insertedJobs[0].JobID)
	assert.Empty(t, insertedJobs[0].Needs)
	assert.Equal(t, []string{"fedora"}, insertedJobs[0].RunsOn)
	assert.Equal(t, int64(1), insertedJobs[0].Attempt)
	assert.Zero(t, insertedJobs[0].Started)
	assert.Zero(t, insertedJobs[0].Stopped)
	assert.Zero(t, insertedJobs[0].TaskID)
	assert.Equal(t, StatusWaiting, insertedJobs[0].Status)
	assert.Len(t, insertedJobs[0].Handle, 36)

	assert.Equal(t, actionRun.ID, insertedJobs[1].RunID)
	assert.Equal(t, actionRun.RepoID, insertedJobs[1].RepoID)
	assert.Equal(t, actionRun.OwnerID, insertedJobs[1].OwnerID)
	assert.Equal(t, actionRun.CommitSHA, insertedJobs[1].CommitSHA)
	assert.Equal(t, actionRun.IsForkPullRequest, insertedJobs[1].IsForkPullRequest)
	assert.Equal(t, "test", insertedJobs[1].Name)
	assert.Equal(t, JobIdentifier("test"), insertedJobs[1].JobID)
	assert.Empty(t, insertedJobs[1].Needs)
	assert.Equal(t, []string{"debian"}, insertedJobs[1].RunsOn)
	assert.Equal(t, int64(1), insertedJobs[1].Attempt)
	assert.Zero(t, insertedJobs[1].Started)
	assert.Zero(t, insertedJobs[1].Stopped)
	assert.Zero(t, insertedJobs[1].TaskID)
	assert.Equal(t, StatusWaiting, insertedJobs[1].Status)
	assert.Equal(t, jobs[1].Handle, insertedJobs[1].Handle)
}

func TestActionRunLoadAttributes(t *testing.T) {
	run := &ActionRun{
		RepoID:        10,
		TriggerUserID: 1000,
	}
	require.NoError(t, run.LoadAttributes(t.Context()))
	assert.Equal(t, "ghost", run.TriggerUser.LowerName)
}

func TestGetRunByID(t *testing.T) {
	const (
		existingRunID    = 0xdeadbeef
		nonexistingRunID = 0xffffffff
	)

	require.NoError(t, unittest.PrepareTestDatabase())

	_, err := db.GetEngine(t.Context()).Insert(ActionRun{
		ID: existingRunID,
	})
	require.NoError(t, err)

	// ActionRun exists

	run, err := GetRunByID(t.Context(), existingRunID)
	require.NoError(t, err)
	assert.NotNil(t, run)

	// ActionRun does not exist

	run, err = GetRunByID(t.Context(), nonexistingRunID)
	require.ErrorIs(t, err, util.ErrNotExist)
	assert.Nil(t, run)
}

func TestGetQueuedRunsByRepoID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	fixtures := []*ActionRun{
		{ID: 535681, Index: 1, RepoID: 62, OwnerID: 2, Status: StatusSuccess},
		{ID: 535682, Index: 2, RepoID: 62, OwnerID: 2, Status: StatusRunning},
		{ID: 535683, Index: 3, RepoID: 62, OwnerID: 2, Status: StatusWaiting},
		{ID: 535684, Index: 4, RepoID: 62, OwnerID: 2, Status: StatusBlocked},
		{ID: 535685, Index: 1, RepoID: 1, OwnerID: 2, Status: StatusBlocked},
		{ID: 535686, Index: 2, RepoID: 1, OwnerID: 2, Status: StatusCancelled},
	}
	unittest.AssertSuccessfulInsert(t, fixtures)

	runs, err := GetQueuedRunsByRepoID(t.Context(), 62)
	require.NoError(t, err)

	assert.Len(t, runs, 2)
	assert.Equal(t, int64(535683), runs[0].ID)
	assert.Equal(t, int64(535684), runs[1].ID)

	runs, err = GetQueuedRunsByRepoID(t.Context(), 1)
	require.NoError(t, err)

	assert.Len(t, runs, 1)
	assert.Equal(t, int64(535685), runs[0].ID)
}

func TestPrepareNextAttempt(t *testing.T) {
	t.Run("Error if pending", func(t *testing.T) {
		for _, pendingStatus := range PendingStatuses() {
			t.Run(pendingStatus.String(), func(t *testing.T) {
				run := &ActionRun{ID: 10, Status: pendingStatus}

				err := run.PrepareNextAttempt()

				require.ErrorContains(t, err, "cannot prepare next attempt because run 10 is active")
			})
		}
	})

	t.Run("Next attempt prepared if done", func(t *testing.T) {
		for _, acceptableStatus := range DoneStatuses() {
			t.Run(acceptableStatus.String(), func(t *testing.T) {
				run := &ActionRun{
					ID:               11,
					Status:           acceptableStatus,
					Started:          1786976036,
					Stopped:          1786976040,
					PreviousDuration: time.Minute,
					Priority:         MaxRunPriority,
					Prioritize:       true,
				}

				require.NoError(t, run.PrepareNextAttempt())

				assert.Equal(t, time.Minute+4*time.Second, run.PreviousDuration)
				assert.Equal(t, StatusWaiting, run.Status)
				assert.Zero(t, run.Started)
				assert.Zero(t, run.Stopped)
				assert.Equal(t, DefaultRunPriority, run.Priority)
				assert.False(t, run.Prioritize)
			})
		}
	})
}

func TestRefreshStatus(t *testing.T) {
	t.Run("Unchanged status", func(t *testing.T) {
		run := &ActionRun{ID: 24, Status: StatusRunning, Stopped: 0}

		assert.False(t, run.RefreshStatus([]*ActionRunJob{{Status: StatusRunning}}))

		assert.Equal(t, StatusRunning, run.Status)
		assert.Zero(t, run.Stopped)
	})

	t.Run("Changed status", func(t *testing.T) {
		run := &ActionRun{ID: 24, Status: StatusBlocked, Stopped: 0}

		assert.True(t, run.RefreshStatus([]*ActionRunJob{{Status: StatusWaiting}}))

		assert.Equal(t, StatusWaiting, run.Status)
		assert.Zero(t, run.Stopped)
	})

	t.Run("Completed", func(t *testing.T) {
		now := time.Now()

		timeutil.MockSet(now)
		defer timeutil.MockUnset()

		run := &ActionRun{ID: 24, Status: StatusBlocked, Stopped: 0}

		assert.True(t, run.RefreshStatus([]*ActionRunJob{{Status: StatusCancelled}}))

		assert.Equal(t, StatusCancelled, run.Status)
		assert.Equal(t, now.Truncate(time.Second), run.Stopped.AsTime())
	})

	t.Run("Unchanged completed status", func(t *testing.T) {
		now := time.Now()

		timeutil.MockSet(now)
		defer timeutil.MockUnset()

		stopped := timeutil.TimeStampNow().Add(-10)

		run := &ActionRun{ID: 24, Status: StatusSuccess, Stopped: stopped}

		assert.False(t, run.RefreshStatus([]*ActionRunJob{{Status: StatusSuccess}}))

		assert.Equal(t, StatusSuccess, run.Status)
		assert.Equal(t, stopped, run.Stopped)
	})
}

func TestUpdateRun(t *testing.T) {
	t.Run("Truncates title", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		run := &ActionRun{ID: 7569, OwnerID: 2, RepoID: 62, Title: "Within limits"}

		unittest.AssertSuccessfulInsert(t, run)

		run.Title = strings.Repeat("m", 256)

		require.NoError(t, UpdateRun(t.Context(), run))

		run = unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID})
		assert.Len(t, run.Title, 255)
	})

	t.Run("Rejects outdated runs", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		originalRun := &ActionRun{ID: 7569, OwnerID: 2, RepoID: 62, Title: "A run"}

		unittest.AssertSuccessfulInsert(t, originalRun)

		runCopy := unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: originalRun.ID})

		assert.Equal(t, 1, originalRun.Version)
		assert.Equal(t, 1, runCopy.Version)

		require.NoError(t, UpdateRun(t.Context(), originalRun))

		assert.Equal(t, 2, originalRun.Version)
		assert.Equal(t, 1, runCopy.Version)

		err := UpdateRun(t.Context(), runCopy)

		assert.ErrorIs(t, err, ErrActionRunOutOfDate)
	})

	t.Run("Updates only given columns", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		run := &ActionRun{ID: 7569, OwnerID: 2, RepoID: 62, Title: "A run"}

		unittest.AssertSuccessfulInsert(t, run)

		unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID, Title: "A run"})

		run.Title = "Changed title"

		require.NoError(t, UpdateRun(t.Context(), run, "id"))

		unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID, Title: "A run"})
		unittest.AssertNotExistsBean(t, &ActionRun{ID: run.ID, Title: "Changed title"})

		require.NoError(t, UpdateRun(t.Context(), run, "title"))

		unittest.AssertNotExistsBean(t, &ActionRun{ID: run.ID, Title: "A run"})
		unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID, Title: "Changed title"})
	})

	t.Run("Updates zero values when not columns given", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		run := &ActionRun{ID: 7569, OwnerID: 2, RepoID: 62, Title: "A run"}

		unittest.AssertSuccessfulInsert(t, run)

		unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID, Title: "A run"})

		run.Title = ""

		require.NoError(t, UpdateRun(t.Context(), run))

		unittest.AssertExistsAndLoadBean(t, &ActionRun{ID: run.ID, Title: ""})
		unittest.AssertNotExistsBean(t, &ActionRun{ID: run.ID, Title: "A run"})
	})

	t.Run("Clears cache", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		require.NoError(t, cache.Init())

		run := &ActionRun{ID: 7569, OwnerID: 2, RepoID: 62, Title: "A run", Status: StatusRunning}

		unittest.AssertSuccessfulInsert(t, run)

		assert.Equal(t, 1, RepoNumOpenActions(t.Context(), run.RepoID))

		run.Status = StatusSuccess

		require.NoError(t, UpdateRun(t.Context(), run))

		assert.Zero(t, RepoNumOpenActions(t.Context(), run.RepoID))
	})
}
