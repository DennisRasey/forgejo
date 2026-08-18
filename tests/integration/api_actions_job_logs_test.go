// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/actions"
	"forgejo.org/modules/json"
	"forgejo.org/modules/setting"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAPIGetActionJobLogs(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip()
	}
	now := time.Now()
	outcome := &mockTaskOutcome{
		result: runnerv1.Result_RESULT_SUCCESS,
		logRows: []*runnerv1.LogRow{
			{Time: timestamppb.New(now.Add(1 * time.Second)), Content: "setup banner line"},
			{Time: timestamppb.New(now.Add(2 * time.Second)), Content: "step error: boom"},
			{Time: timestamppb.New(now.Add(3 * time.Second)), Content: "complete banner line"},
		},
		// Real step (Id=0) covers only the middle log row. Lines [0] and [2]
		// end up in FullSteps's setup head and complete tail respectively,
		// giving the ?step= tests something distinguishable for each kind.
		stepStates: []*runnerv1.StepState{
			{
				Id:        0,
				Result:    runnerv1.Result_RESULT_SUCCESS,
				LogIndex:  1,
				LogLength: 1,
				StartedAt: timestamppb.New(now),
				StoppedAt: timestamppb.New(now.Add(3 * time.Second)),
			},
		},
	}
	workflow := `name: api-job-logs
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
`
	treePath := ".forgejo/workflows/api-job-logs.yml"

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session,
			auth_model.AccessTokenScopeWriteRepository,
			auth_model.AccessTokenScopeWriteUser,
		)

		// Repo A receives the workflow + runs the job.
		apiRepoA := createActionsTestRepo(t, token, "actions-job-logs", false)
		repoA := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepoA.ID})

		// Repo B is the cross-repo target — used to verify the guard.
		apiRepoB := createActionsTestRepo(t, token, "actions-job-logs-other", false)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repoA.Name, "mock-runner", []string{"ubuntu-latest"})

		opts := getWorkflowCreateFileOptions(user2, repoA.DefaultBranch,
			fmt.Sprintf("create %s", treePath), workflow)
		createWorkflowFile(t, token, user2.Name, repoA.Name, treePath, opts)

		task := runner.fetchTask(t)
		runner.execTask(t, task, outcome)

		actionTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.Id})
		jobID := actionTask.JobID

		t.Run("happy path: 200 plaintext", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)

			require.Contains(t, resp.Header().Get("Content-Type"), "text/plain")
			assert.Equal(t, "bytes", resp.Header().Get("Accept-Ranges"))

			lines := strings.Split(strings.TrimSpace(resp.Body.String()), "\n")
			require.Len(t, lines, len(outcome.logRows))
			for i, lr := range outcome.logRows {
				assert.Equal(t, actions.FormatLog(lr.Time.AsTime(), lr.Content), lines[i])
			}
		})

		t.Run("cross-repo: 404 when job_id belongs to a different repo", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs",
				apiRepoB.FullName, jobID,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("not found: 404 for unknown job_id", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs",
				repoA.FullName(), jobID+999999,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("wrong scope: 403 without read:repository", func(t *testing.T) {
			// Token with only user scope, no repository access.
			weakToken := getTokenForLoggedInUser(t, session,
				auth_model.AccessTokenScopeReadUser,
			)
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(weakToken)
			MakeRequest(t, req, http.StatusForbidden)
		})

		t.Run("range: 206 partial content", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			req.Header.Set("Range", "bytes=0-15")
			resp := MakeRequest(t, req, http.StatusPartialContent)
			// [0,15] inclusive = 16 bytes max.
			assert.LessOrEqual(t, resp.Body.Len(), 16)
		})

		t.Run("attempt=1: 200 matches no-param (only attempt)", func(t *testing.T) {
			defaultReq := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs",
				repoA.FullName(), jobID,
			)
			defaultReq.AddTokenAuth(token)
			defaultResp := MakeRequest(t, defaultReq, http.StatusOK)

			attemptReq := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?attempt=1",
				repoA.FullName(), jobID,
			)
			attemptReq.AddTokenAuth(token)
			attemptResp := MakeRequest(t, attemptReq, http.StatusOK)

			assert.Equal(t, defaultResp.Body.String(), attemptResp.Body.String())
		})

		t.Run("attempt=0: 404 (no row in DB)", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?attempt=0",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("attempt=999: 404 unknown attempt", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?attempt=999",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		// FullSteps numbering: 0 = "Set up job" head, 1 = the one real step,
		// 2 = "Complete job" tail. With stepStates above, each gets exactly
		// one log line. step=99 is out of range.

		t.Run("step=0: 200 setup head only", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?step=0",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			body := strings.TrimSpace(resp.Body.String())
			require.NotEmpty(t, body, "setup head should contain the first log line")
			assert.Contains(t, body, "setup banner line")
			assert.NotContains(t, body, "step error: boom")
			assert.NotContains(t, body, "complete banner line")
		})

		t.Run("step=1: 200 real step only", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?step=1",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			body := strings.TrimSpace(resp.Body.String())
			assert.Contains(t, body, "step error: boom")
			assert.NotContains(t, body, "setup banner line")
			assert.NotContains(t, body, "complete banner line")
		})

		t.Run("step=2: 200 complete tail only", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?step=2",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			body := strings.TrimSpace(resp.Body.String())
			assert.Contains(t, body, "complete banner line")
			assert.NotContains(t, body, "setup banner line")
			assert.NotContains(t, body, "step error: boom")
		})

		t.Run("step=99: 404 out of range", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?step=99",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("format=ndjson: 200 NDJSON every line", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/jobs/%d/logs?format=ndjson",
				repoA.FullName(), jobID,
			)
			req.AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Header().Get("Content-Type"), "application/x-ndjson")
			assert.Empty(t, resp.Header().Get("Accept-Ranges"))

			lines := strings.Split(strings.TrimRight(resp.Body.String(), "\n"), "\n")
			require.Len(t, lines, len(outcome.logRows))

			type jsonLine struct {
				Time    time.Time `json:"time"`
				Content string    `json:"content"`
			}
			var l0 jsonLine
			require.NoError(t, json.Unmarshal([]byte(lines[0]), &l0))
			assert.Equal(t, "setup banner line", l0.Content)
			assert.False(t, l0.Time.IsZero())
		})

		httpContextA := NewAPITestContext(t, user2.Name, repoA.Name, auth_model.AccessTokenScopeWriteUser)
		doAPIDeleteRepository(httpContextA)(t)
		httpContextB := NewAPITestContext(t, user2.Name, apiRepoB.Name, auth_model.AccessTokenScopeWriteUser)
		doAPIDeleteRepository(httpContextB)(t)
	})
}
