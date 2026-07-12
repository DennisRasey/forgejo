// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/models/user"
	actions_module "forgejo.org/modules/actions"
	"forgejo.org/modules/json"
	"forgejo.org/modules/setting"
	webhook_module "forgejo.org/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindTaskNeeds(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 51})
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})

	ret, err := FindTaskNeeds(t.Context(), job)
	require.NoError(t, err)
	assert.Len(t, ret, 1)
	assert.Contains(t, ret, "job1")
	assert.Len(t, ret["job1"].Outputs, 2)
	assert.Equal(t, "abc", ret["job1"].Outputs["output_a"])
	assert.Equal(t, "bbb", ret["job1"].Outputs["output_b"])
}

func TestGenerateGiteaContext(t *testing.T) {
	testUser := &user.User{
		ID:   1,
		Name: "testuser",
	}

	testRepo := &repo.Repository{
		ID:        1,
		OwnerName: "testowner",
		Name:      "testrepo",
	}

	emptyField := func(t *testing.T, context map[string]any, field string) {
		v, ok := context[field]
		assert.True(t, ok, "expected field %q to be present", field)
		assert.Empty(t, v)
	}

	t.Run("Basic workflow run without job", func(t *testing.T) {
		run := &actions_model.ActionRun{
			ID:                1,
			Index:             42,
			TriggerUser:       testUser,
			Repo:              testRepo,
			TriggerEvent:      "push",
			Ref:               "refs/heads/main",
			CommitSHA:         "abc123def456",
			WorkflowID:        "test-workflow.yaml",
			WorkflowDirectory: ".forgejo/workflows",
			EventPayload:      `{"repository": {"name": "testrepo"}}`,
		}

		context, err := GenerateGiteaContext(run, nil)
		require.NoError(t, err)

		assert.Equal(t, "testuser", context["actor"])
		assert.Equal(t, setting.AppURL+"api/v1", context["api_url"])
		assert.Equal(t, "push", context["event_name"])
		assert.Equal(t, "refs/heads/main", context["ref"])
		assert.Equal(t, "main", context["ref_name"])
		assert.Equal(t, "branch", context["ref_type"])
		assert.Equal(t, "testowner/testrepo", context["repository"])
		assert.Equal(t, "testowner", context["repository_owner"])
		assert.Equal(t, "abc123def456", context["sha"])
		assert.Equal(t, "42", context["run_number"])
		assert.Equal(t, "test-workflow.yaml", context["workflow"])
		assert.Equal(t, "testowner/testrepo/.forgejo/workflows/test-workflow.yaml@refs/heads/main", context["workflow_ref"])
		assert.Equal(t, false, context["ref_protected"])
		assert.Equal(t, "Actions", context["secret_source"])
		assert.Equal(t, setting.AppURL, context["server_url"])
		assert.Equal(t, setting.Actions.DefaultActionsURL.URL(), context["gitea_default_actions_url"])
		assert.Equal(t, setting.Actions.DefaultActionsURL.URL(), context["forgejo_default_actions_url"])
		assert.Equal(t, setting.AppVer, context["forgejo_server_version"])

		event, ok := context["event"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "testrepo", event["repository"].(map[string]any)["name"])

		emptyField(t, context, "action_path")
		emptyField(t, context, "action_ref")
		emptyField(t, context, "action_repository")
		emptyField(t, context, "action_status")
		emptyField(t, context, "action")
		emptyField(t, context, "base_ref")
		emptyField(t, context, "env")
		emptyField(t, context, "event_path")
		emptyField(t, context, "graphql_url")
		emptyField(t, context, "head_ref")
		emptyField(t, context, "job")
		emptyField(t, context, "path")
		emptyField(t, context, "retention_days")
		emptyField(t, context, "run_attempt")
		emptyField(t, context, "run_id")
		emptyField(t, context, "triggering_actor")
		emptyField(t, context, "workspace")
	})

	t.Run("Workflow run with job", func(t *testing.T) {
		run := &actions_model.ActionRun{
			ID:           1,
			Index:        42,
			TriggerUser:  testUser,
			Repo:         testRepo,
			TriggerEvent: "push",
			Ref:          "refs/heads/main",
			CommitSHA:    "abc123def456",
			WorkflowID:   "test-workflow",
			EventPayload: `{}`,
		}

		job := &actions_model.ActionRunJob{
			ID:              100,
			RunID:           1,
			JobID:           "test-job",
			Attempt:         1,
			WorkflowPayload: []byte("on: [push]"),
		}

		context, err := GenerateGiteaContext(run, job)
		require.NoError(t, err)

		assert.Equal(t, "test-job", context["job"])
		assert.Equal(t, "1", context["run_id"])
		assert.Equal(t, "1", context["run_attempt"])
	})

	t.Run("Pull request event", func(t *testing.T) {
		pullRequestPayload := map[string]any{
			"pull_request": map[string]any{
				"base": map[string]any{
					"ref":   "main",
					"label": "main",
					"sha":   "base123sha",
				},
				"head": map[string]any{
					"ref":   "feature-branch",
					"label": "feature-branch",
					"sha":   "head456sha",
				},
			},
		}

		payloadBytes, _ := json.Marshal(pullRequestPayload)

		run := &actions_model.ActionRun{
			ID:                1,
			Index:             42,
			TriggerUser:       testUser,
			Repo:              testRepo,
			TriggerEvent:      "pull_request",
			Ref:               "refs/pull/1/merge",
			CommitSHA:         "merge789sha",
			WorkflowID:        "test-workflow.yaml",
			WorkflowDirectory: ".forgejo/workflows",
			Event:             webhook_module.HookEventPullRequest,
			EventPayload:      string(payloadBytes),
		}

		context, err := GenerateGiteaContext(run, nil)
		require.NoError(t, err)

		assert.Equal(t, "main", context["base_ref"])
		assert.Equal(t, "feature-branch", context["head_ref"])
		assert.Equal(t, "refs/pull/1/merge", context["ref"])
		assert.Equal(t, "merge789sha", context["sha"])
		assert.Equal(t, "testowner/testrepo/.forgejo/workflows/test-workflow.yaml@refs/pull/1/merge", context["workflow_ref"])
	})

	t.Run("Pull request target event", func(t *testing.T) {
		pullRequestPayload := map[string]any{
			"pull_request": map[string]any{
				"base": map[string]any{
					"ref":   "main",
					"label": "main",
					"sha":   "base123sha",
				},
				"head": map[string]any{
					"ref":   "feature-branch",
					"label": "feature-branch",
					"sha":   "head456sha",
				},
			},
		}

		payloadBytes, _ := json.Marshal(pullRequestPayload)

		run := &actions_model.ActionRun{
			ID:                1,
			Index:             42,
			TriggerUser:       testUser,
			Repo:              testRepo,
			TriggerEvent:      actions_module.GithubEventPullRequestTarget,
			Ref:               "refs/pull/1/merge",
			CommitSHA:         "merge789sha",
			WorkflowID:        "test-workflow.yml",
			WorkflowDirectory: ".github/workflows",
			Event:             webhook_module.HookEventPullRequest,
			EventPayload:      string(payloadBytes),
		}

		context, err := GenerateGiteaContext(run, nil)
		require.NoError(t, err)

		assert.Equal(t, "main", context["base_ref"])
		assert.Equal(t, "feature-branch", context["head_ref"])
		// For pull_request_target, ref and sha should be from base
		assert.Equal(t, "refs/heads/main", context["ref"])
		assert.Equal(t, "base123sha", context["sha"])
		assert.Equal(t, "main", context["ref_name"])
		assert.Equal(t, "branch", context["ref_type"])
		assert.Equal(t, "testowner/testrepo/.github/workflows/test-workflow.yml@refs/heads/main", context["workflow_ref"])
	})

	t.Run("workflow_call job", func(t *testing.T) {
		run := &actions_model.ActionRun{
			ID:           1,
			Index:        42,
			TriggerUser:  testUser,
			Repo:         testRepo,
			TriggerEvent: "push",
			Ref:          "refs/heads/main",
			CommitSHA:    "abc123def456",
			WorkflowID:   "test-workflow",
			EventPayload: `{}`,
		}

		job := &actions_model.ActionRunJob{
			ID:              100,
			RunID:           1,
			JobID:           "test-job",
			Attempt:         1,
			WorkflowPayload: []byte("on: { workflow_call: { inputs: {} } }\n__metadata:\n  workflow_call_parent: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed\n"),
		}

		context, err := GenerateGiteaContext(run, job)
		require.NoError(t, err)

		assert.Equal(t, "workflow_call", context["event_name"])
	})
}

func TestGenerateGiteaContextForRun(t *testing.T) {
	testUser := &user.User{
		ID:   1,
		Name: "testuser",
	}

	testRepo := &repo.Repository{
		ID:        1,
		OwnerName: "testowner",
		Name:      "testrepo",
	}

	t.Run("Basic workflow run", func(t *testing.T) {
		run := &actions_model.ActionRun{
			ID:                1,
			Index:             42,
			TriggerUser:       testUser,
			Repo:              testRepo,
			TriggerEvent:      "push",
			Ref:               "refs/heads/main",
			CommitSHA:         "abc123def456",
			WorkflowID:        "test-workflow.yaml",
			WorkflowDirectory: ".forgejo/workflows",
			EventPayload:      `{"repository": {"name": "testrepo"}}`,
		}

		gitContextObj := generateGiteaContextForRun(run)

		assert.Equal(t, "testuser", gitContextObj.Actor)
		assert.Equal(t, setting.AppURL+"api/v1", gitContextObj.APIURL)
		assert.Equal(t, "push", gitContextObj.EventName)
		assert.Equal(t, "refs/heads/main", gitContextObj.Ref)
		assert.Equal(t, "main", gitContextObj.RefName)
		assert.Equal(t, "branch", gitContextObj.RefType)
		assert.Equal(t, "testowner/testrepo", gitContextObj.Repository)
		assert.Equal(t, "testowner", gitContextObj.RepositoryOwner)
		assert.Equal(t, "abc123def456", gitContextObj.Sha)
		assert.Equal(t, "42", gitContextObj.RunNumber)
		assert.Equal(t, "test-workflow.yaml", gitContextObj.Workflow)
		assert.Equal(t, "testowner/testrepo/.forgejo/workflows/test-workflow.yaml@refs/heads/main", gitContextObj.WorkflowRef)

		assert.Equal(t, "testrepo", gitContextObj.Event["repository"].(map[string]any)["name"])

		assert.Empty(t, gitContextObj.ActionPath)
		assert.Empty(t, gitContextObj.ActionRef)
		assert.Empty(t, gitContextObj.ActionRepository)
		assert.Empty(t, gitContextObj.Action)
		assert.Empty(t, gitContextObj.BaseRef)
		assert.Empty(t, gitContextObj.EventPath)
		assert.Empty(t, gitContextObj.GraphQLURL)
		assert.Empty(t, gitContextObj.HeadRef)
		assert.Empty(t, gitContextObj.Job)
		assert.Empty(t, gitContextObj.RetentionDays)
		assert.Empty(t, gitContextObj.RunAttempt)
		assert.Empty(t, gitContextObj.RunID)
		assert.Empty(t, gitContextObj.Workspace)
	})

	t.Run("Pull request event", func(t *testing.T) {
		pullRequestPayload := map[string]any{
			"pull_request": map[string]any{
				"base": map[string]any{
					"ref":   "main",
					"label": "main",
					"sha":   "base123sha",
				},
				"head": map[string]any{
					"ref":   "feature-branch",
					"label": "feature-branch",
					"sha":   "head456sha",
				},
			},
		}

		payloadBytes, _ := json.Marshal(pullRequestPayload)

		run := &actions_model.ActionRun{
			ID:                1,
			Index:             42,
			TriggerUser:       testUser,
			Repo:              testRepo,
			TriggerEvent:      "pull_request",
			Ref:               "refs/pull/1/merge",
			CommitSHA:         "merge789sha",
			WorkflowID:        "test-workflow.yaml",
			WorkflowDirectory: ".forgejo/workflows",
			Event:             webhook_module.HookEventPullRequest,
			EventPayload:      string(payloadBytes),
		}

		gitContextObj := generateGiteaContextForRun(run)

		assert.Equal(t, "main", gitContextObj.BaseRef)
		assert.Equal(t, "feature-branch", gitContextObj.HeadRef)
		assert.Equal(t, "refs/pull/1/merge", gitContextObj.Ref)
		assert.Equal(t, "merge789sha", gitContextObj.Sha)
		assert.Equal(t, "testowner/testrepo/.forgejo/workflows/test-workflow.yaml@refs/pull/1/merge", gitContextObj.WorkflowRef)
	})

	t.Run("Pull request target event", func(t *testing.T) {
		pullRequestPayload := map[string]any{
			"pull_request": map[string]any{
				"base": map[string]any{
					"ref":   "main",
					"label": "main",
					"sha":   "base123sha",
				},
				"head": map[string]any{
					"ref":   "feature-branch",
					"label": "feature-branch",
					"sha":   "head456sha",
				},
			},
		}

		payloadBytes, _ := json.Marshal(pullRequestPayload)

		run := &actions_model.ActionRun{
			ID:                1,
			Index:             42,
			TriggerUser:       testUser,
			Repo:              testRepo,
			TriggerEvent:      actions_module.GithubEventPullRequestTarget,
			Ref:               "refs/pull/1/merge",
			CommitSHA:         "merge789sha",
			WorkflowID:        "test-workflow.yml",
			WorkflowDirectory: ".github/workflows",
			Event:             webhook_module.HookEventPullRequest,
			EventPayload:      string(payloadBytes),
		}

		gitContextObj := generateGiteaContextForRun(run)

		assert.Equal(t, "main", gitContextObj.BaseRef)
		assert.Equal(t, "feature-branch", gitContextObj.HeadRef)
		// For pull_request_target, ref and sha should be from base
		assert.Equal(t, "refs/heads/main", gitContextObj.Ref)
		assert.Equal(t, "base123sha", gitContextObj.Sha)
		assert.Equal(t, "main", gitContextObj.RefName)
		assert.Equal(t, "branch", gitContextObj.RefType)
		assert.Equal(t, "testowner/testrepo/.github/workflows/test-workflow.yml@refs/heads/main", gitContextObj.WorkflowRef)
	})
}
