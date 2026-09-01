// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAPIGetActionRunLogs(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip()
	}
	now := time.Now()
	outcomeJob1 := &mockTaskOutcome{
		result: runnerv1.Result_RESULT_SUCCESS,
		logRows: []*runnerv1.LogRow{
			{Time: timestamppb.New(now.Add(1 * time.Second)), Content: "job1-output line one"},
			{Time: timestamppb.New(now.Add(2 * time.Second)), Content: "job1-output line two"},
		},
	}
	outcomeJob2 := &mockTaskOutcome{
		result: runnerv1.Result_RESULT_SUCCESS,
		logRows: []*runnerv1.LogRow{
			{Time: timestamppb.New(now.Add(3 * time.Second)), Content: "job2-output line one"},
			{Time: timestamppb.New(now.Add(4 * time.Second)), Content: "job2-output line two"},
		},
	}
	// A third job with a non-ASCII display name (kanji + emoji) to confirm
	// the ZIP entry preserves UTF-8 verbatim — regression guard against any
	// future tightening of the filename sanitize.
	outcomeJobUTF8 := &mockTaskOutcome{
		result: runnerv1.Result_RESULT_SUCCESS,
		logRows: []*runnerv1.LogRow{
			{Time: timestamppb.New(now.Add(5 * time.Second)), Content: "utf8-output line one"},
		},
	}
	// Display name uses kanji + a non-BMP emoji (U+1F680) so the test
	// exercises multi-byte UTF-8 and surrogate-pair territory.
	utf8JobDisplayName := "测试-\U0001f680"
	workflow := fmt.Sprintf(`name: api-run-logs
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1 first line
  job2:
    runs-on: ubuntu-latest
    needs: [job1]
    steps:
      - run: echo job2 first line
  utf8-job:
    name: %s
    runs-on: ubuntu-latest
    needs: [job2]
    steps:
      - run: echo utf8 first line
`, utf8JobDisplayName)
	treePath := ".forgejo/workflows/api-run-logs.yml"

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session,
			auth_model.AccessTokenScopeWriteRepository,
			auth_model.AccessTokenScopeWriteUser,
		)

		// Repo A receives the workflow + runs the jobs.
		apiRepoA := createActionsTestRepo(t, token, "actions-run-logs", false)
		repoA := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepoA.ID})

		// Repo B is the cross-repo target — used to verify the guard.
		apiRepoB := createActionsTestRepo(t, token, "actions-run-logs-other", false)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repoA.Name, "mock-runner", []string{"ubuntu-latest"})

		opts := getWorkflowCreateFileOptions(user2, repoA.DefaultBranch,
			fmt.Sprintf("create %s", treePath), workflow)
		createWorkflowFile(t, token, user2.Name, repoA.Name, treePath, opts)

		// Dependency chain: job1 → job2 → utf8-job. Each `needs:` clause
		// makes the runner pickup order deterministic.
		task1 := runner.fetchTask(t)
		runner.execTask(t, task1, outcomeJob1)

		task2 := runner.fetchTask(t)
		runner.execTask(t, task2, outcomeJob2)

		task3 := runner.fetchTask(t)
		runner.execTask(t, task3, outcomeJobUTF8)

		actionTask1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task1.Id})
		actionRunJob1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask1.JobID})
		actionTask2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task2.Id})
		actionRunJob2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask2.JobID})
		actionTask3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task3.Id})
		actionRunJob3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask3.JobID})

		require.Equal(t, actions_model.JobIdentifier("job1"), actionRunJob1.JobID, "first fetched task should be job1")
		require.Equal(t, actions_model.JobIdentifier("job2"), actionRunJob2.JobID, "second fetched task should be job2 (needs: [job1])")
		require.Equal(t, actions_model.JobIdentifier("utf8-job"), actionRunJob3.JobID, "third fetched task should be utf8-job (needs: [job2])")
		require.Equal(t, utf8JobDisplayName, actionRunJob3.Name,
			"DB should store the YAML `name:` field verbatim, including UTF-8")
		require.Equal(t, actionRunJob1.RunID, actionRunJob2.RunID, "both jobs should belong to the same run")
		require.Equal(t, actionRunJob1.RunID, actionRunJob3.RunID, "utf8-job should belong to the same run")
		runID := actionRunJob1.RunID

		t.Run("happy path: 200 valid zip with per-job entries", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/runs/%d/logs",
				repoA.FullName(), runID,
			)
			req.AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, "application/zip", resp.Header().Get("Content-Type"))
			assert.Contains(t, resp.Header().Get("Content-Disposition"),
				fmt.Sprintf("run-%d-logs.zip", runID))

			r, err := zip.NewReader(bytes.NewReader(resp.Body.Bytes()), int64(resp.Body.Len()))
			require.NoError(t, err)

			// Read every entry into a map keyed by filename so we can assert
			// names and content exactly, and surface unexpected extras.
			entries := map[string]string{}
			for _, f := range r.File {
				fr, err := f.Open()
				require.NoError(t, err)
				data, err := io.ReadAll(fr)
				require.NoError(t, err)
				require.NoError(t, fr.Close())
				entries[f.Name] = string(data)
			}

			job1Name := fmt.Sprintf("%s-%d-attempt-%d.log", actionRunJob1.Name, actionRunJob1.ID, actionRunJob1.Attempt)
			job2Name := fmt.Sprintf("%s-%d-attempt-%d.log", actionRunJob2.Name, actionRunJob2.ID, actionRunJob2.Attempt)
			utf8Name := fmt.Sprintf("%s-%d-attempt-%d.log", actionRunJob3.Name, actionRunJob3.ID, actionRunJob3.Attempt)

			require.Len(t, entries, 3, "zip should contain exactly one entry per job")
			require.Contains(t, entries, job1Name, "expected job1 entry %q", job1Name)
			require.Contains(t, entries, job2Name, "expected job2 entry %q", job2Name)
			// Explicit UTF-8-preservation check: the entry name must contain
			// the verbatim kanji + emoji, with no Unicode-to-underscore mangling.
			require.Contains(t, entries, utf8Name,
				"expected UTF-8 entry %q (sanitize must preserve non-ASCII verbatim)", utf8Name)

			assert.Contains(t, entries[job1Name], "job1-output line one")
			assert.Contains(t, entries[job1Name], "job1-output line two")
			assert.Contains(t, entries[job2Name], "job2-output line one")
			assert.Contains(t, entries[job2Name], "job2-output line two")
			assert.Contains(t, entries[utf8Name], "utf8-output line one")
		})

		t.Run("cross-repo: 404 when run_id belongs to a different repo", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/runs/%d/logs",
				apiRepoB.FullName, runID,
			)
			req.AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("not found: 404 for unknown run_id", func(t *testing.T) {
			req := NewRequestf(t, "GET",
				"/api/v1/repos/%s/actions/runs/%d/logs",
				repoA.FullName(), runID+999999,
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
				"/api/v1/repos/%s/actions/runs/%d/logs",
				repoA.FullName(), runID,
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
