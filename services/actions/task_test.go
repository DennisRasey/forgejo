package actions

import (
	"fmt"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/repo"
	"forgejo.org/models/user"

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

	testRepo := &repo.Repository{
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

		taskContext, err := generateTaskContext(task)
		require.NoError(t, err)
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})

	t.Run("openid connect enabled from fork with pull_request_target event", func(t *testing.T) {
		task := createTask(fmt.Sprintf(workflowFormat, "true"), true, "pull_request_target")

		taskContext, err := generateTaskContext(task)
		require.NoError(t, err)
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})

	t.Run("openid connect enabled from fork with pull_request event", func(t *testing.T) {
		task := createTask(fmt.Sprintf(workflowFormat, "true"), true, "pull_request")

		taskContext, err := generateTaskContext(task)
		require.NoError(t, err)
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})

	t.Run("openid connect disabled", func(t *testing.T) {
		task := createTask(fmt.Sprintf(workflowFormat, "false"), false, "push")

		taskContext, err := generateTaskContext(task)
		require.NoError(t, err)
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_token"].GetStringValue())
		require.Empty(t, taskContext.Fields["forgejo_actions_id_token_request_url"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["gitea_runtime_token"].GetStringValue())
		require.NotEmpty(t, taskContext.Fields["forgejo_runtime_token"].GetStringValue())
	})
}
