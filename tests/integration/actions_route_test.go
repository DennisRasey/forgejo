// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	files_service "code.gitea.io/gitea/services/repository/files"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestActionsWebRouteLatestWorkflowRun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, _, f := CreateDeclarativeRepo(t, user2, "",
			[]unit_model.Type{unit_model.TypeActions}, nil,
			[]*files_service.ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      ".gitea/workflows/workflow-1.yml",
					ContentReader: strings.NewReader("name: workflow-1\non:\n  push:\njobs:\n  job-1:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
				},
				{
					Operation:     "create",
					TreePath:      ".gitea/workflows/workflow-2.yml",
					ContentReader: strings.NewReader("name: workflow-2\non:\n  push:\njobs:\n  job-2:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo helloworld\n"),
				},
			},
		)
		defer f()

		t.Run("valid workflows", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// helpers
			getWorkflowRunRedirectURI := func(workflow string) string {
				req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/workflows/%s/runs/latest", repo.HTMLURL(), workflow))
				resp := MakeRequest(t, req, http.StatusTemporaryRedirect)

				return resp.Header().Get("Location")
			}

			// two runs have been created
			assert.Equal(t, 2, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))

			// Get the redirect URIs for both workflows
			workflowOneURI := getWorkflowRunRedirectURI("workflow-1.yml")
			workflowTwoURI := getWorkflowRunRedirectURI("workflow-2.yml")

			// Verify that the two are different.
			assert.NotEqual(t, workflowOneURI, workflowTwoURI)

			// Verify that each points to the correct workflow.
			workflowOne := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, Index: 1})
			err := workflowOne.LoadAttributes(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, workflowOneURI, workflowOne.HTMLURL())

			workflowTwo := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, Index: 2})
			err = workflowTwo.LoadAttributes(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, workflowTwoURI, workflowTwo.HTMLURL())
		})

		t.Run("existing workflow, non-existent branch", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/workflows/workflow-1.yml/runs/latest?branch=foobar", repo.HTMLURL()))
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("non-existing workflow", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/workflows/workflow-3.yml/runs/latest", repo.HTMLURL()))
			MakeRequest(t, req, http.StatusNotFound)
		})
	})
}

func TestActionsWebRouteLatestRun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, _, f := CreateDeclarativeRepo(t, user2, "",
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

		// Hit the `/actions/runs/latest` route
		req := NewRequest(t, "GET", fmt.Sprintf("%s/actions/runs/latest", repo.HTMLURL()))
		resp := MakeRequest(t, req, http.StatusTemporaryRedirect)

		// Verify that it redirects to the run we just created
		workflow := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID})
		err := workflow.LoadAttributes(context.Background())
		assert.NoError(t, err)

		assert.Equal(t, workflow.HTMLURL(), resp.Header().Get("Location"))
	})
}
