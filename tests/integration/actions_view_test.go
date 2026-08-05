// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	actions_model "forgejo.org/models/actions"
	repo_model "forgejo.org/models/repo"
	unit_model "forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"
	files_service "forgejo.org/services/repository/files"
	"forgejo.org/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionViewsArtifactDeletion(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, _, f := tests.CreateDeclarativeRepo(t, user2, "",
			[]unit_model.Type{unit_model.TypeActions}, nil,
			[]*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      ".gitea/workflows/pr.yml",
					ContentReader: strings.NewReader("name: test\non:\n  push:\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
				},
			},
		)
		defer f()

		// a run has been created
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))

		// Load the run we just created
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID})
		err := run.LoadAttributes(t.Context())
		require.NoError(t, err)

		// Visit it's web view
		req := NewRequest(t, "GET", run.HTMLURL())
		intermediateRedirect := MakeRequest(t, req, http.StatusTemporaryRedirect)

		finalURL := intermediateRedirect.Result().Header.Get("Location")
		req = NewRequest(t, "GET", finalURL)
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		// Assert that the artifact deletion markup exists
		htmlDoc.AssertElement(t, "[data-locale-confirm-delete-artifact]", true)
	})
}

func TestActionViewsArtifactDownload(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	assertDataAttrs := func(t *testing.T, body *bytes.Buffer, runID int64) {
		t.Helper()
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
		htmlDoc := NewHTMLParser(t, body)
		selector := "#repo-action-view"
		htmlDoc.AssertAttrEqual(t, selector, "data-run-id", fmt.Sprintf("%d", run.ID))
		htmlDoc.AssertAttrEqual(t, selector, "data-run-index", fmt.Sprintf("%d", run.Index))
	}

	t.Run("V3", func(t *testing.T) {
		runIndex := 187
		runID := int64(791)

		req := NewRequest(t, "GET", fmt.Sprintf("/user5/repo4/actions/runs/%d/artifacts", runIndex))
		resp := MakeRequest(t, req, http.StatusOK)
		assert.JSONEq(t, `{"artifacts":[{"name":"multi-file-download","size":2048,"status":"completed"}]}`, strings.TrimSuffix(resp.Body.String(), "\n"))

		req = NewRequest(t, "GET", fmt.Sprintf("/user5/repo4/actions/runs/%d", runIndex))
		intermediateRedirect := MakeRequest(t, req, http.StatusTemporaryRedirect)

		finalURL := intermediateRedirect.Result().Header.Get("Location")
		req = NewRequest(t, "GET", finalURL)
		resp = MakeRequest(t, req, http.StatusOK)
		assertDataAttrs(t, resp.Body, runID)

		req = NewRequest(t, "GET", fmt.Sprintf("/user5/repo4/actions/runs/%d/artifacts/multi-file-download", runID))
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Header().Get("content-disposition"), "multi-file-download.zip")
	})

	t.Run("V4", func(t *testing.T) {
		runIndex := 188
		runID := int64(792)

		req := NewRequest(t, "GET", fmt.Sprintf("/user5/repo4/actions/runs/%d/artifacts", runIndex))
		resp := MakeRequest(t, req, http.StatusOK)
		assert.JSONEq(t, `{"artifacts":[{"name":"artifact-v4-download","size":1024,"status":"completed"}]}`, strings.TrimSuffix(resp.Body.String(), "\n"))

		req = NewRequest(t, "GET", fmt.Sprintf("/user5/repo4/actions/runs/%d", runIndex))
		intermediateRedirect := MakeRequest(t, req, http.StatusTemporaryRedirect)

		finalURL := intermediateRedirect.Result().Header.Get("Location")
		req = NewRequest(t, "GET", finalURL)
		resp = MakeRequest(t, req, http.StatusOK)
		assertDataAttrs(t, resp.Body, runID)

		download := fmt.Sprintf("/user5/repo4/actions/runs/%d/artifacts/artifact-v4-download", runID)
		req = NewRequest(t, "GET", download)
		resp = MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "bytes", resp.Header().Get("accept-ranges"))
		assert.Contains(t, resp.Header().Get("content-disposition"), "artifact-v4-download.zip")
		assert.Equal(t, strings.Repeat("D", 1024), resp.Body.String())

		// Partial artifact download
		req = NewRequest(t, "GET", download).SetHeader("range", "bytes=0-99")
		resp = MakeRequest(t, req, http.StatusPartialContent)
		assert.Equal(t, "bytes 0-99/1024", resp.Header().Get("content-range"))
		assert.Equal(t, strings.Repeat("D", 100), resp.Body.String())
	})
}

func TestActionViewsView(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionViewsView")()
	defer tests.PrepareTestEnv(t)()

	parisTZ, err := time.LoadLocation("Europe/Paris")
	require.NoError(t, err)

	defer test.MockVariableValue(&setting.DefaultUILocation, parisTZ)()

	timeutil.MockSet(time.Date(2023, 5, 9, 13, 21, 32, 0, time.UTC))
	defer timeutil.MockUnset()

	testCases := []struct {
		name              string
		url               string
		runIndex          int64
		jobIndex          int64
		attempt           int64
		workflowSourceURL string
		expectedJSON      string
		expectedArtifacts string
	}{
		{
			name:              "push",
			url:               "/user5/repo4/actions/runs/187",
			runIndex:          187,
			jobIndex:          0,
			attempt:           3,
			workflowSourceURL: "/user5/repo4/src/commit/c2d72f548424103f01ee1dc02889c1e2bff816b0/.forgejo/workflows/artifact.yaml",
			expectedJSON:      "{\"state\":{\"run\":{\"preExecutionError\":\"\",\"preExecutionWarnings\":null,\"link\":\"/user5/repo4/actions/runs/187\",\"title\":\"update actions\",\"titleHTML\":\"update actions\",\"status\":\"success\",\"canCancel\":false,\"canDelete\":false,\"canApprove\":false,\"canRerun\":false,\"canDeleteArtifact\":false,\"description\":\"Commit <a href=\\\"/user5/repo4/commit/c2d72f548424103f01ee1dc02889c1e2bff816b0\\\">c2d72f5484</a> pushed by <a href=\\\"/user1\\\">user1</a>\",\"done\":true,\"jobs\":[{\"id\":192,\"name\":\"job_2\",\"status\":\"success\",\"canRerun\":false,\"duration\":\"_duration_\"}],\"commit\":{\"localeWorkflow\":\"Workflow\",\"localeAllRuns\":\"all runs\",\"shortSHA\":\"c2d72f5484\",\"link\":\"/user5/repo4/commit/c2d72f548424103f01ee1dc02889c1e2bff816b0\",\"pusher\":{\"displayName\":\"user1\",\"link\":\"/user1\"},\"branch\":{\"name\":\"master\",\"link\":\"/user5/repo4/src/branch/master\",\"isDeleted\":false}}},\"currentJob\":{\"title\":\"job_2\",\"steps\":[{\"summary\":\"Set up job\",\"duration\":\"_duration_\",\"status\":\"running\"},{\"summary\":\"Complete job\",\"duration\":\"_duration_\",\"status\":\"waiting\"}],\"allAttempts\":[{\"number\":3,\"time_since_started_html\":\"_time_\",\"status\":\"running\",\"status_diagnostics\":[\"Running for <relative-time datetime=\\\"2023-05-09T14:48:48+02:00\\\" format=\\\"duration\\\" prefix=\\\"\\\" data-tooltip-content data-tooltip-interactive=\\\"true\\\">32m44s</relative-time>\"]},{\"number\":2,\"time_since_started_html\":\"_time_\",\"status\":\"success\",\"status_diagnostics\":[\"Succeeded <relative-time prefix=\\\"\\\" tense=\\\"past\\\" datetime=\\\"2023-05-09T14:50:26+02:00\\\" data-tooltip-content data-tooltip-interactive=\\\"true\\\">2023-05-09 14:50:26 +02:00</relative-time> in 1m38s\"]},{\"number\":1,\"time_since_started_html\":\"_time_\",\"status\":\"success\",\"status_diagnostics\":[\"Succeeded <relative-time prefix=\\\"\\\" tense=\\\"past\\\" datetime=\\\"2023-05-09T14:50:26+02:00\\\" data-tooltip-content data-tooltip-interactive=\\\"true\\\">2023-05-09 14:50:26 +02:00</relative-time> in 1m38s\"]}]}},\"logs\":{\"stepsLog\":[]}}\n",
			expectedArtifacts: "{\"artifacts\":[{\"name\":\"multi-file-download\",\"size\":2048,\"status\":\"completed\"}]}\n",
		},
		{
			name:              "scheduled",
			url:               "/user5/repo4/actions/runs/209",
			runIndex:          209,
			jobIndex:          0,
			attempt:           1,
			workflowSourceURL: "/user5/repo4/src/commit/64357baca84bfff631e7dfae5a3433b26d005646/.forgejo/workflows/scheduled.yaml",
			expectedJSON:      "{\"state\":{\"run\":{\"link\":\"/user5/repo4/actions/runs/209\",\"title\":\"A scheduled workflow\",\"titleHTML\":\"A scheduled workflow\",\"status\":\"waiting\",\"description\":\"Scheduled run of commit \\u003ca href=\\\"/user5/repo4/commit/64357baca84bfff631e7dfae5a3433b26d005646\\\"\\u003e64357baca8\\u003c/a\\u003e\",\"canCancel\":false,\"canDelete\":false,\"canApprove\":false,\"canRerun\":false,\"canDeleteArtifact\":false,\"done\":false,\"jobs\":[{\"id\":2153,\"name\":\"job_2\",\"status\":\"waiting\",\"canRerun\":false,\"duration\":\"_duration_\"}],\"commit\":{\"localeWorkflow\":\"Workflow\",\"localeAllRuns\":\"all runs\",\"shortSHA\":\"64357baca8\",\"link\":\"/user5/repo4/commit/64357baca84bfff631e7dfae5a3433b26d005646\",\"pusher\":{\"displayName\":\"forgejo-actions\",\"link\":\"/forgejo-actions\"},\"branch\":{\"name\":\"master\",\"link\":\"/user5/repo4/src/branch/master\",\"isDeleted\":false}},\"preExecutionError\":\"\",\"preExecutionWarnings\":null},\"currentJob\":{\"title\":\"job_2\",\"steps\":[{\"summary\":\"Set up job\",\"duration\":\"_duration_\",\"status\":\"running\"},{\"summary\":\"Complete job\",\"duration\":\"_duration_\",\"status\":\"waiting\"}],\"allAttempts\":[{\"number\":1,\"time_since_started_html\":\"-\",\"status\":\"waiting\",\"status_diagnostics\":[\"Waiting for <relative-time datetime=\\\"2023-05-09T14:47:08+02:00\\\" format=\\\"duration\\\" prefix=\\\"\\\" data-tooltip-content data-tooltip-interactive=\\\"true\\\">34m24s</relative-time> for a runner with the following labels: debian, gpu\"]}]}},\"logs\":{\"stepsLog\":[]}}\n",
			expectedArtifacts: "{\"artifacts\":[]}\n",
		},
		{
			name:              "workflow_dispatch",
			url:               "/user5/repo4/actions/runs/210",
			runIndex:          210,
			jobIndex:          0,
			attempt:           1,
			workflowSourceURL: "/user5/repo4/src/commit/f4100ac14112a3740490afb22b07b69b0b5d4e8b/.forgejo/workflows/dispatch.yaml",
			expectedJSON:      "{\"state\":{\"run\":{\"link\":\"/user5/repo4/actions/runs/210\",\"title\":\"A triggered run\",\"titleHTML\":\"A triggered run\",\"status\":\"waiting\",\"description\":\"Run of commit \\u003ca href=\\\"/user5/repo4/commit/f4100ac14112a3740490afb22b07b69b0b5d4e8b\\\"\\u003ef4100ac141\\u003c/a\\u003e triggered by \\u003ca href=\\\"/user29\\\"\\u003euser29\\u003c/a\\u003e\",\"canCancel\":false,\"canDelete\":false,\"canApprove\":false,\"canRerun\":false,\"canDeleteArtifact\":false,\"done\":false,\"jobs\":[{\"id\":2154,\"name\":\"mirror\",\"status\":\"waiting\",\"canRerun\":false,\"duration\":\"_duration_\"}],\"commit\":{\"localeWorkflow\":\"Workflow\",\"localeAllRuns\":\"all runs\",\"shortSHA\":\"f4100ac141\",\"link\":\"/user5/repo4/commit/f4100ac14112a3740490afb22b07b69b0b5d4e8b\",\"pusher\":{\"displayName\":\"user29\",\"link\":\"/user29\"},\"branch\":{\"name\":\"master\",\"link\":\"/user5/repo4/src/branch/master\",\"isDeleted\":false}},\"preExecutionError\":\"\",\"preExecutionWarnings\":null},\"currentJob\":{\"title\":\"mirror\",\"steps\":[{\"summary\":\"Set up job\",\"duration\":\"_duration_\",\"status\":\"running\"},{\"summary\":\"Complete job\",\"duration\":\"_duration_\",\"status\":\"waiting\"}],\"allAttempts\":[{\"number\":1,\"time_since_started_html\":\"-\",\"status\":\"waiting\",\"status_diagnostics\":[\"Waiting for - for a runner with the following label: windows\"]}]}},\"logs\":{\"stepsLog\":[]}}\n",
			expectedArtifacts: "{\"artifacts\":[]}\n",
		},
		{
			name:              "pull_request_target",
			url:               "/user5/repo4/actions/runs/211",
			runIndex:          211,
			jobIndex:          0,
			attempt:           1,
			workflowSourceURL: "/user5/repo4/src/commit/deadbeef/.forgejo/workflows/pull_request_target.yaml",
			expectedJSON:      "{\"state\":{\"run\":{\"link\":\"/user5/repo4/actions/runs/211\",\"title\":\"A pull_request_target run\",\"titleHTML\":\"A pull_request_target run\",\"status\":\"waiting\",\"description\":\"Commit \\u003ca href=\\\"/user5/repo4/commit/f4100ac14112a3740490afb22b07b69b0b5d4e8b\\\"\\u003ef4100ac141\\u003c/a\\u003e pushed by \\u003ca href=\\\"/user29\\\"\\u003euser29\\u003c/a\\u003e\",\"canCancel\":false,\"canApprove\":false,\"canRerun\":false,\"canDeleteArtifact\":false,\"canDelete\":false,\"done\":false,\"jobs\":[{\"id\":2155,\"name\":\"target_job\",\"status\":\"waiting\",\"canRerun\":false,\"duration\":\"_duration_\"}],\"commit\":{\"localeWorkflow\":\"Workflow\",\"localeAllRuns\":\"all runs\",\"shortSHA\":\"f4100ac141\",\"link\":\"/user5/repo4/commit/f4100ac14112a3740490afb22b07b69b0b5d4e8b\",\"pusher\":{\"displayName\":\"user29\",\"link\":\"/user29\"},\"branch\":{\"name\":\"master\",\"link\":\"/user5/repo4/src/branch/master\",\"isDeleted\":false}},\"preExecutionError\":\"\",\"preExecutionWarnings\":null},\"currentJob\":{\"title\":\"target_job\",\"steps\":[{\"summary\":\"Set up job\",\"duration\":\"_duration_\",\"status\":\"running\"},{\"summary\":\"Complete job\",\"duration\":\"_duration_\",\"status\":\"waiting\"}],\"allAttempts\":[{\"number\":1,\"time_since_started_html\":\"-\",\"status\":\"waiting\",\"status_diagnostics\":[\"Waiting for - for a runner with the following label: macos\"]}]}},\"logs\":{\"stepsLog\":[]}}\n",
			expectedArtifacts: "{\"artifacts\":[]}\n",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			req := NewRequest(t, "GET", testCase.url)
			intermediateRedirect := MakeRequest(t, req, http.StatusTemporaryRedirect)

			finalURL := intermediateRedirect.Result().Header.Get("Location")
			req = NewRequest(t, "GET", finalURL)
			resp := MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)
			selector := "#repo-action-view"
			// Verify key properties going into the `repo-action-view` to initialize the Vue component.
			htmlDoc.AssertAttrEqual(t, selector, "data-run-index", strconv.FormatInt(testCase.runIndex, 10))
			htmlDoc.AssertAttrEqual(t, selector, "data-job-index", strconv.FormatInt(testCase.jobIndex, 10))
			htmlDoc.AssertAttrEqual(t, selector, "data-attempt-number", strconv.FormatInt(testCase.attempt, 10))
			htmlDoc.AssertAttrEqual(t, selector, "data-workflow-source-url", testCase.workflowSourceURL)
			htmlDoc.AssertAttrPredicate(t, selector, "data-initial-post-response", func(actual string) {
				// Remove dynamic "duration" fields for comparison.
				pattern := `"duration":"[^"]*"`
				re := regexp.MustCompile(pattern)
				actualClean := re.ReplaceAllString(actual, `"duration":"_duration_"`)
				// Remove "time_since_started_html" fields for comparison since they're TZ-sensitive in the test
				pattern = `"time_since_started_html":".*?\\u003c/relative-time\\u003e"`
				re = regexp.MustCompile(pattern)
				actualClean = re.ReplaceAllString(actualClean, `"time_since_started_html":"_time_"`)

				assert.JSONEq(t, testCase.expectedJSON, actualClean)
			})
			htmlDoc.AssertAttrEqual(t, selector, "data-initial-artifacts-response", testCase.expectedArtifacts)
		})
	}
}

// Action re-run will redirect the user to an attempt that may not exist in the database yet, since attempts are only
// updated in the DB when jobs are picked up by runners.  This test is intended to ensure that a "future" attempt number
// can still be loaded into the repo-action-view, which will handle waiting & polling for it to have data.
func TestActionViewsViewAttemptOutOfRange(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// For this test to accurately reflect an attempt not yet picked, it needs to be accessing an ActionRunJob with
	// TaskID: null... otherwise we can't fetch future unpersisted attempts.
	req := NewRequest(t, "GET", "/user5/repo4/actions/runs/190/jobs/0/attempt/100")
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	selector := "#repo-action-view"
	// Verify key properties going into the `repo-action-view` to initialize the Vue component.
	htmlDoc.AssertAttrEqual(t, selector, "data-run-index", "190")
	htmlDoc.AssertAttrEqual(t, selector, "data-job-index", "0")
	htmlDoc.AssertAttrEqual(t, selector, "data-attempt-number", "100")
	htmlDoc.AssertAttrPredicate(t, selector, "data-initial-post-response", func(actual string) {
		// Remove dynamic "duration" fields for comparison.
		pattern := `"duration":"[^"]*"`
		re := regexp.MustCompile(pattern)
		actualClean := re.ReplaceAllString(actual, `"duration":"_duration_"`)
		// Remove "time_since_started_html" fields for comparison since they're TZ-sensitive in the test
		pattern = `"time_since_started_html":".*?\\u003c/relative-time\\u003e"`
		re = regexp.MustCompile(pattern)
		actualClean = re.ReplaceAllString(actualClean, `"time_since_started_html":"_time_"`)

		assert.JSONEq(t, "{\"state\":{\"run\":{\"preExecutionError\":\"\",\"preExecutionWarnings\":null,\"link\":\"/user5/repo4/actions/runs/190\",\"title\":\"job output\",\"titleHTML\":\"job output\",\"status\":\"success\",\"canCancel\":false,\"canDelete\":false,\"canApprove\":false,\"canRerun\":false,\"canDeleteArtifact\":false,\"description\":\"Commit <a href=\\\"/user5/repo4/commit/c2d72f548424103f01ee1dc02889c1e2bff816b0\\\">c2d72f5484</a> pushed by <a href=\\\"/user1\\\">user1</a>\",\"done\":false,\"jobs\":[{\"id\":396,\"name\":\"job_2\",\"status\":\"waiting\",\"canRerun\":false,\"duration\":\"_duration_\"}],\"commit\":{\"localeWorkflow\":\"Workflow\",\"localeAllRuns\":\"all runs\",\"shortSHA\":\"c2d72f5484\",\"link\":\"/user5/repo4/commit/c2d72f548424103f01ee1dc02889c1e2bff816b0\",\"pusher\":{\"displayName\":\"user1\",\"link\":\"/user1\"},\"branch\":{\"name\":\"test\",\"link\":\"/user5/repo4/src/branch/test\",\"isDeleted\":true}}},\"currentJob\":{\"title\":\"job_2\",\"steps\":[],\"allAttempts\":[{\"number\":1,\"status\":\"waiting\",\"status_diagnostics\":[\"Waiting for - for a runner with the following label: fedora\"],\"time_since_started_html\":\"not started\"}]}},\"logs\":{\"stepsLog\":[]}}\n", actualClean)
	})
	htmlDoc.AssertAttrEqual(t, selector, "data-initial-artifacts-response", "{\"artifacts\":[]}\n")
}

func TestActionTabAccessibleFromRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElementPredicate(t, "a[href='/user2/repo1/actions']", func(selection *goquery.Selection) {
		text := strings.TrimSpace(selection.Text())
		assert.Contains(t, text, "Actions")
	})

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user2.Name)
	req = NewRequest(t, "GET", "/user2/test_action_run_search/actions")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElementPredicate(t, "a[href='/user2/test_action_run_search/actions']", func(selection *goquery.Selection) {
		text := strings.TrimSpace(selection.Text())
		assert.Contains(t, text, "Actions")
		assert.Contains(t, text, "1") // This repo has one running action run
	})
}

func TestActionViewRunDeletion(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionViewRunDeletion")()
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})

	sessionUser2 := loginUser(t, user2.Name)
	sessionUser5 := loginUser(t, user5.Name)

	isCollaborator, err := repo_model.IsCollaborator(t.Context(), repo1.ID, user5.ID)
	require.NoError(t, err)
	require.True(t, isCollaborator)

	testCases := []struct {
		name       string
		sess       *TestSession
		requestURL string
		canDelete  bool
	}{
		{
			name:       "Repo owner can delete completed run",
			sess:       sessionUser2,
			requestURL: fmt.Sprintf("/%s/actions/runs/3161/jobs/0/attempt/1", repo1.FullName()),
			canDelete:  true,
		},
		{
			name:       "Collaborator cannot delete completed run",
			sess:       sessionUser5,
			requestURL: fmt.Sprintf("/%s/actions/runs/3161/jobs/0/attempt/1", repo1.FullName()),
			canDelete:  false,
		},
		{
			name:       "Repo owner cannot delete running run",
			sess:       sessionUser2,
			requestURL: fmt.Sprintf("/%s/actions/runs/3162/jobs/0/attempt/1", repo1.FullName()),
			canDelete:  false,
		},
		{
			name:       "Collaborator cannot delete running run",
			sess:       sessionUser5,
			requestURL: fmt.Sprintf("/%s/actions/runs/3162/jobs/0/attempt/1", repo1.FullName()),
			canDelete:  false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			req := NewRequest(t, "GET", testCase.requestURL)
			resp := testCase.sess.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			htmlDoc.AssertAttrPredicate(t, "#repo-action-view", "data-initial-post-response", func(actual string) {
				assert.Contains(t, actual, fmt.Sprintf(`"canDelete":%t`, testCase.canDelete))
			})
		})
	}
}
