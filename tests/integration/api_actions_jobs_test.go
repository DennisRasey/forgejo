// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/json"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAPIGetActionJob(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip()
	}
	now := time.Now()
	outcome := &mockTaskOutcome{
		result: runnerv1.Result_RESULT_SUCCESS,
		logRows: []*runnerv1.LogRow{
			{Time: timestamppb.New(now.Add(1 * time.Second)), Content: "hello from job"},
		},
		stepStates: []*runnerv1.StepState{
			{
				Id:        0,
				Result:    runnerv1.Result_RESULT_SUCCESS,
				LogIndex:  0,
				LogLength: 1,
				StartedAt: timestamppb.New(now),
				StoppedAt: timestamppb.New(now.Add(2 * time.Second)),
			},
		},
	}
	workflow := `name: api-job
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello from job
`
	treePath := ".forgejo/workflows/api-job.yml"

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session,
			auth_model.AccessTokenScopeWriteRepository,
			auth_model.AccessTokenScopeWriteUser,
		)

		apiRepoA := createActionsTestRepo(t, token, "actions-jobs-api", false)
		repoA := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepoA.ID})

		apiRepoB := createActionsTestRepo(t, token, "actions-jobs-api-other", false)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repoA.Name, "mock-runner", []string{"ubuntu-latest"})

		opts := getWorkflowCreateFileOptions(user2, repoA.DefaultBranch,
			fmt.Sprintf("create %s", treePath), workflow)
		createWorkflowFile(t, token, user2.Name, repoA.Name, treePath, opts)

		task := runner.fetchTask(t)
		runner.execTask(t, task, outcome)

		actionTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.Id})
		jobID := actionTask.JobID

		t.Run("happy path: 200 with steps array", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)

			var got api.ActionRunJob
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &got))

			assert.Equal(t, jobID, got.ID)
			assert.Equal(t, "job1", got.Name)

			// FullSteps shape: [setup, real, complete] — exactly 3 entries
			// for a job with one real workflow step.
			require.Len(t, got.Steps, 3, "expected setup + real + complete")

			assert.Equal(t, int64(0), got.Steps[0].Number)
			assert.Equal(t, "Set up job", got.Steps[0].Name)

			assert.Equal(t, int64(1), got.Steps[1].Number)
			// Real step name comes from the workflow YAML — runner returns the
			// `run:` command as a default name when no `name:` is set.
			assert.NotEmpty(t, got.Steps[1].Name)
			assert.NotEqual(t, "Set up job", got.Steps[1].Name)
			assert.NotEqual(t, "Complete job", got.Steps[1].Name)

			assert.Equal(t, int64(2), got.Steps[2].Number)
			assert.Equal(t, "Complete job", got.Steps[2].Name)
		})

		t.Run("cross-repo: 404 when job_id belongs to a different repo", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d",
				apiRepoB.FullName, jobID,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("not found: 404 for unknown job_id", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d",
				repoA.FullName(), jobID+999999,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("wrong scope: 403 without read:repository", func(t *testing.T) {
			weakToken := getTokenForLoggedInUser(t, session,
				auth_model.AccessTokenScopeReadUser,
			)
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(weakToken)
			MakeRequest(t, req, http.StatusForbidden)
		})

		httpContextA := NewAPITestContext(t, user2.Name, repoA.Name, auth_model.AccessTokenScopeWriteUser)
		doAPIDeleteRepository(httpContextA)(t)
		httpContextB := NewAPITestContext(t, user2.Name, apiRepoB.Name, auth_model.AccessTokenScopeWriteUser)
		doAPIDeleteRepository(httpContextB)(t)
	})
}
