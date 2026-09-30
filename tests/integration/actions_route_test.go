// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "forgejo.org/models/actions"
	unit_model "forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func GetWorkflowRunRedirectURI(t *testing.T, repoURL, workflow string) string {
	t.Helper()

	req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/workflows/%s/runs/latest", repoURL, workflow))
	resp := MakeRequest(t, req, http.StatusTemporaryRedirect)

	return resp.Header().Get("Location")
}

func TestActionsWebRouteLatestWorkflowRun(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{
			Name: "actionsTestRepo",
			Files: forgery.MapFS{
				".gitea/workflows/workflow-1.yml": forgery.MapFile("name: workflow-1\non:\n  push:\njobs:\n  job-1:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
				".gitea/workflows/workflow-2.yml": forgery.MapFile("name: workflow-2\non:\n  push:\njobs:\n  job-2:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
			},
		})
		forgery.EnableRepoUnit(t, repo, unit_model.TypeActions, nil)

		repoURL := repo.HTMLURL()

		t.Run("valid workflows", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// two runs have been created
			assert.Equal(t, 2, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))

			// Get the redirect URIs for both workflows
			workflowOneURI := GetWorkflowRunRedirectURI(t, repoURL, "workflow-1.yml")
			workflowTwoURI := GetWorkflowRunRedirectURI(t, repoURL, "workflow-2.yml")

			// Verify that the two are different.
			assert.NotEqual(t, workflowOneURI, workflowTwoURI)

			// Verify that each points to the correct workflow.
			workflowOne := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, Index: 1})
			err := workflowOne.LoadAttributes(t.Context())
			require.NoError(t, err)
			assert.Equal(t, workflowOneURI, workflowOne.HTMLURL())

			workflowTwo := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, Index: 2})
			err = workflowTwo.LoadAttributes(t.Context())
			require.NoError(t, err)
			assert.Equal(t, workflowTwoURI, workflowTwo.HTMLURL())
		})

		t.Run("check if workflow page shows file name", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Get the redirect URI
			workflow := "workflow-1.yml"
			workflowOneURI := GetWorkflowRunRedirectURI(t, repoURL, workflow)

			// Fetch the page that shows information about the run initiated by "workflow-1.yml".
			// routers/web/repo/actions/view.go: data-workflow-url is constructed using data-workflow-name.
			req := NewRequest(t, "GET", workflowOneURI)
			intermediateRedirect := MakeRequest(t, req, http.StatusTemporaryRedirect)

			finalURL := intermediateRedirect.Result().Header.Get("Location")
			req = NewRequest(t, "GET", finalURL)
			resp := MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)

			// Verify that URL of the workflow is shown correctly.
			expectedURL := fmt.Sprintf("/user2/actionsTestRepo/actions?workflow=%s", workflow)
			htmlDoc.AssertElement(t, fmt.Sprintf("#repo-action-view[data-workflow-url=\"%s\"]", expectedURL), true)
		})

		t.Run("existing workflow, non-existent branch", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/workflows/workflow-1.yml/runs/latest?branch=foobar", repoURL))
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("non-existing workflow", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/workflows/workflow-3.yml/runs/latest", repoURL))
			MakeRequest(t, req, http.StatusNotFound)
		})
	})
}

func TestActionsWebRouteLatestRun(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{
				".gitea/workflows/pr.yml": forgery.MapFile("name: test\non:\n  push:\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
			},
		})
		forgery.EnableRepoUnit(t, repo, unit_model.TypeActions, nil)

		// a run has been created
		assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))

		// Hit the `/actions/runs/latest` route
		req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/runs/latest", repo.HTMLURL()))
		resp := MakeRequest(t, req, http.StatusTemporaryRedirect)

		// Verify that it redirects to the run we just created
		workflow := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID})
		err := workflow.LoadAttributes(t.Context())
		require.NoError(t, err)

		assert.Equal(t, workflow.HTMLURL(), resp.Header().Get("Location"))
	})
}
