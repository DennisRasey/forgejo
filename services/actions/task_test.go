package actions

import (
	"fmt"
	"testing"

	actions_model "forgejo.org/models/actions"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/models/user"
	"forgejo.org/modules/actions"
	notify_service "forgejo.org/services/notify"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGenerateTaskContext(t *testing.T) {
	workflowFormat := `
name: Pull Request
on: pull_request
enable-openid-connect: %s
jobs:
  wf1-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'test the pull'
`
	testUser := &user.User{
		ID:   1,
		Name: "testuser",
	}

	testRepo := &repo_model.Repository{
		ID:        1,
		OwnerName: "testowner",
		Name:      "testrepo",
	}

	createTask := func(workflowPayload string, isFork bool, triggerEvent string) *actions_model.ActionTask {
		return &actions_model.ActionTask{
			ID: 47,
			Job: &actions_model.ActionRunJob{
				ID:    2,
				RunID: 1,
				Run: &actions_model.ActionRun{
					ID:                1,
					Index:             42,
					TriggerUser:       testUser,
					Repo:              testRepo,
					TriggerEvent:      triggerEvent,
					Ref:               "refs/heads/main",
					CommitSHA:         "abc123def456",
					WorkflowID:        "test-workflow.yaml",
					WorkflowDirectory: ".forgejo/workflows",
					EventPayload:      `{"repository": {"name": "testrepo"}}`,
					IsForkPullRequest: isFork,
				},
				WorkflowPayload: []byte(workflowPayload),
			},
		}
	}

	t.Run("openid connect enabled", func(t *testing.T) {
		task := createTask(fmt.Sprintf(workflowFormat, "true"), false, "push")

		taskContext, err := generateTaskContext(task, &repo_model.ActionsConfig{})
		require.NoError(t, err)
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})

	t.Run("openid connect enabled from fork with pull_request_target event", func(t *testing.T) {
		task := createTask(fmt.Sprintf(workflowFormat, "true"), true, "pull_request_target")

		taskContext, err := generateTaskContext(task, &repo_model.ActionsConfig{})
		require.NoError(t, err)
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})

	t.Run("openid connect enabled from fork with pull_request event", func(t *testing.T) {
		task := createTask(fmt.Sprintf(workflowFormat, "true"), true, "pull_request")

		taskContext, err := generateTaskContext(task, &repo_model.ActionsConfig{})
		require.NoError(t, err)
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})

	t.Run("openid connect disabled", func(t *testing.T) {
		task := createTask(fmt.Sprintf(workflowFormat, "false"), false, "push")

		taskContext, err := generateTaskContext(task, &repo_model.ActionsConfig{})
		require.NoError(t, err)
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})
}

func TestDeleteTask(t *testing.T) {
	t.Run("Task removed with logs and ephemeral runner", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87601})
		runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: 41601})
		unittest.AssertCount(t, &actions_model.ActionTaskOutput{TaskID: task.ID}, 2)
		unittest.AssertCount(t, &actions_model.ActionTaskStep{TaskID: task.ID}, 1)

		_, err := actions.WriteLogs(t.Context(), task.LogFilename, 0, []*runnerv1.LogRow{{Content: "OK"}})
		require.NoError(t, err)

		logExists, err := actions.ExistsLogs(t.Context(), task.LogFilename)
		require.NoError(t, err)
		assert.True(t, logExists)

		require.NoError(t, deleteTask(t.Context(), task.ID))

		logExists, err = actions.ExistsLogs(t.Context(), task.LogFilename)
		require.NoError(t, err)
		assert.False(t, logExists)

		unittest.AssertNotExistsBean(t, &actions_model.ActionTask{ID: task.ID})
		unittest.AssertCount(t, &actions_model.ActionTaskOutput{TaskID: task.ID}, 0)
		unittest.AssertCount(t, &actions_model.ActionTaskStep{TaskID: task.ID}, 0)
		unittest.AssertNotExistsBean(t, &actions_model.ActionRunner{ID: runner.ID})

		// Verify that other tasks have been left alone.
		otherTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87602})
		unittest.AssertCount(t, &actions_model.ActionTaskOutput{TaskID: otherTask.ID}, 1)
		unittest.AssertCount(t, &actions_model.ActionTaskStep{TaskID: otherTask.ID}, 1)
	})

	t.Run("Task removed and persistent runner kept", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87603})
		runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: 41602})
		unittest.AssertCount(t, &actions_model.ActionTaskOutput{TaskID: task.ID}, 0)
		unittest.AssertCount(t, &actions_model.ActionTaskStep{TaskID: task.ID}, 1)

		_, err := actions.WriteLogs(t.Context(), task.LogFilename, 0, []*runnerv1.LogRow{{Content: "OK"}})
		require.NoError(t, err)

		logExists, err := actions.ExistsLogs(t.Context(), task.LogFilename)
		require.NoError(t, err)
		assert.True(t, logExists)

		require.NoError(t, deleteTask(t.Context(), task.ID))

		logExists, err = actions.ExistsLogs(t.Context(), task.LogFilename)
		require.NoError(t, err)
		assert.False(t, logExists)

		unittest.AssertNotExistsBean(t, &actions_model.ActionTask{ID: task.ID})
		unittest.AssertCount(t, &actions_model.ActionTaskOutput{TaskID: task.ID}, 0)
		unittest.AssertCount(t, &actions_model.ActionTaskStep{TaskID: task.ID}, 0)
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: runner.ID})
	})

	t.Run("Task without logs removed", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87604})
		assert.Empty(t, task.LogFilename)

		require.NoError(t, deleteTask(t.Context(), task.ID))

		unittest.AssertNotExistsBean(t, &actions_model.ActionTask{ID: task.ID})
	})

	t.Run("No error if task does not exist", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())

		unittest.AssertNotExistsBean(t, &actions_model.ActionTask{ID: 87601})
		require.NoError(t, deleteTask(t.Context(), 87601))
	})

	t.Run("Error if task is not done", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestDeleteTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87602})
		unittest.AssertCount(t, &actions_model.ActionTaskOutput{TaskID: task.ID}, 1)
		unittest.AssertCount(t, &actions_model.ActionTaskStep{TaskID: task.ID}, 1)

		err := deleteTask(t.Context(), task.ID)
		require.ErrorContains(t, err, "unable to remove task 87602 because it has not completed yet")

		// Verify nothing has been deleted.
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.ID})
		unittest.AssertCount(t, &actions_model.ActionTaskOutput{TaskID: task.ID}, 1)
		unittest.AssertCount(t, &actions_model.ActionTaskStep{TaskID: task.ID}, 1)
	})
}

func TestStopTask(t *testing.T) {
	t.Run("Invalid status", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestStopTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87601})

		err := StopTask(t.Context(), task.ID, actions_model.StatusRunning)
		require.ErrorContains(t, err, "new task status running is not acceptable")
	})

	t.Run("Completed task", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestStopTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87601})
		assert.Equal(t, actions_model.StatusFailure, task.Status)

		require.NoError(t, StopTask(t.Context(), task.ID, actions_model.StatusCancelled))

		task = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87601})
		assert.Equal(t, actions_model.StatusFailure, task.Status)
	})

	t.Run("Task stopped and runner removed", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestStopTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return()
		notifier.On("WorkflowRunCompleted", mock.Anything, mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87602})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
		runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: task.RunnerID})

		require.NoError(t, StopTask(t.Context(), task.ID, actions_model.StatusCancelled))

		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID, Status: actions_model.StatusCancelled})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID, Status: actions_model.StatusCancelled})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.ID, Status: actions_model.StatusCancelled})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTaskStep{TaskID: task.ID, Status: actions_model.StatusCancelled})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRunner{ID: runner.ID})

		notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 1)
		notifier.AssertNumberOfCalls(t, "WorkflowRunCompleted", 1)
		notifier.AssertCalled(
			t, "WorkflowJobCompleted", mock.Anything,
			mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
				return job.ID == task.JobID && job.Status == actions_model.StatusCancelled
			}),
			actions_model.StatusWaiting,
		)
		notifier.AssertCalled(
			t, "WorkflowRunCompleted", mock.Anything,
			mock.MatchedBy(func(run *actions_model.ActionRun) bool {
				return run.ID == 34902 && run.Status == actions_model.StatusCancelled
			}),
			actions_model.StatusWaiting,
		)
	})

	t.Run("Task stopped and runner retained", func(t *testing.T) {
		defer unittest.OverrideFixtures("services/actions/TestStopTask")()
		require.NoError(t, unittest.PrepareTestDatabase())

		notifier := notify_service.NewMockNotifier(t)
		notifier.On("Run").Return().Maybe()
		notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return()
		notifier.On("WorkflowRunCompleted", mock.Anything, mock.Anything, mock.Anything).Return()

		notify_service.RegisterNotifier(notifier)
		defer notify_service.UnregisterNotifier(notifier)

		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 87603})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})
		runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: task.RunnerID})

		require.NoError(t, StopTask(t.Context(), task.ID, actions_model.StatusCancelled))

		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: job.RunID, Status: actions_model.StatusCancelled})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID, Status: actions_model.StatusCancelled})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.ID, Status: actions_model.StatusCancelled})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTaskStep{TaskID: task.ID, Status: actions_model.StatusCancelled})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: runner.ID})

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
}
