// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "forgejo.org/models/auth"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/json"
	"forgejo.org/modules/setting"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsStepSummaries(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip()
	}
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-step-summaries", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		treePath := ".forgejo/workflows/step-summaries.yml"
		fileContent := `name: step-summaries
on:
  push:
    paths:
      - '.forgejo/workflows/step-summaries.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo '# step one' >> $GITHUB_STEP_SUMMARY
      - run: echo 'step two' >> $GITHUB_STEP_SUMMARY
`
		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", treePath), fileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, treePath, opts)

		task := runner.fetchTask(t)
		runIndex := task.Context.GetFields()["run_number"].GetStringValue()
		viewURL := fmt.Sprintf("/%s/%s/actions/runs/%s/jobs/0/attempt/1", user2.Name, repo.Name, runIndex)

		pollSummaries := func(t *testing.T) []string {
			t.Helper()
			req := NewRequestWithJSON(t, "POST", viewURL, map[string]any{"logCursors": []any{}})
			resp := session.MakeRequest(t, req, http.StatusOK)
			var view struct {
				State struct {
					CurrentJob struct {
						Summaries []string `json:"summaries"`
					} `json:"currentJob"`
				} `json:"state"`
			}
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &view))
			return view.State.CurrentJob.Summaries
		}
		uploadSummaries := func(t *testing.T, summaries ...*runnerv1.StepSummary) {
			t.Helper()
			_, err := runner.client.runnerServiceClient.UpdateStepSummary(t.Context(), connect.NewRequest(&runnerv1.UpdateStepSummaryRequest{
				TaskId:    task.Id,
				Summaries: summaries,
			}))
			require.NoError(t, err)
		}

		// the summary returns empty before any step uploads one
		assert.Empty(t, pollSummaries(t))

		// the summary of the first step becomes visible while the job is still running
		uploadSummaries(t, &runnerv1.StepSummary{StepNumber: 0, Content: "# heading of step one"})
		summaries := pollSummaries(t)
		require.Len(t, summaries, 1)
		assert.Contains(t, summaries[0], "heading of step one")

		// The second step's summary grows the list. Each step's summary is rendered as its own markdown
		// document, so the broken fence gets closed by the parser at the end of its own block and cannot
		// bleed into the other summaries of the RepoActionView.
		// Resending the unchanged first summary simulates a batched/retried flush: the runner only uploads
		// summaries it considers dirty, but always with their full content, so a resend upserts the row
		// instead of duplicating it.
		uploadSummaries(t,
			&runnerv1.StepSummary{StepNumber: 0, Content: "# heading of step one"},
			&runnerv1.StepSummary{StepNumber: 1, Content: "```\nunclosed fence of step two"},
		)
		summaries = pollSummaries(t)
		require.Len(t, summaries, 2)
		assert.Contains(t, summaries[0], "heading of step one")
		assert.NotContains(t, summaries[0], "unclosed fence")
		assert.Contains(t, summaries[1], "unclosed fence of step two")

		// the summaries stay visible once the job has finished
		runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		summaries = pollSummaries(t)
		require.Len(t, summaries, 2)
		assert.Contains(t, summaries[0], "heading of step one")
		assert.Contains(t, summaries[1], "unclosed fence of step two")
	})
}
