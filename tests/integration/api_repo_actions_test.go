// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	unit_model "forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/util"
	"forgejo.org/modules/webhook"
	"forgejo.org/routers/api/v1/shared"
	repo_service "forgejo.org/services/repository"
	files_service "forgejo.org/services/repository/files"
	"forgejo.org/tests"

	gouuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsAPISearchActionJobs_RepoRunner(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequestf(
		t,
		"GET",
		"/api/v1/repos/%s/%s/actions/runners/jobs?labels=%s",
		repo.OwnerName, repo.Name,
		"ubuntu-latest",
	).AddTokenAuth(token)
	res := MakeRequest(t, req, http.StatusOK)

	var jobs []*api.ActionRunJob
	DecodeJSON(t, res, &jobs)

	job393 := api.ActionRunJob{
		ID:      393,
		RunID:   891,
		Attempt: 1,
		Handle:  "18e9cf40-c2f6-409f-b832-b945ea7dc79b",
		RepoID:  1,
		OwnerID: 1,
		Name:    "job_2",
		Needs:   nil,
		RunsOn:  []string{"ubuntu-latest"},
		TaskID:  47,
		Status:  "waiting",
	}

	assert.ElementsMatch(t, []*api.ActionRunJob{&job393}, jobs)
}

func TestActionsAPISearchActionJobs_RepoRunnerAllPendingJobsWithoutLabels(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 196})

	req := NewRequestf(
		t,
		"GET",
		"/api/v1/repos/%s/%s/actions/runners/jobs?labels=",
		repo.OwnerName, repo.Name,
	).AddTokenAuth(token)
	res := MakeRequest(t, req, http.StatusOK)

	var jobs []*api.ActionRunJob
	DecodeJSON(t, res, &jobs)

	assert.Len(t, jobs, 1)
	assert.Equal(t, job.ID, jobs[0].ID)
}

func TestActionsAPISearchActionJobs_RepoRunnerAllPendingJobs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)
	job393 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 393})
	job394 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 394})
	job395 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 395})
	job397 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 397})

	req := NewRequestf(
		t,
		"GET",
		"/api/v1/repos/%s/%s/actions/runners/jobs",
		repo.OwnerName, repo.Name,
	).AddTokenAuth(token)
	res := MakeRequest(t, req, http.StatusOK)

	var jobs []*api.ActionRunJob
	DecodeJSON(t, res, &jobs)

	assert.Len(t, jobs, 4)
	assert.Equal(t, job397.ID, jobs[0].ID)
	assert.Equal(t, job395.ID, jobs[1].ID)
	assert.Equal(t, job394.ID, jobs[2].ID)
	assert.Equal(t, job393.ID, jobs[3].ID)
}

func TestActionsAPIWorkflowDispatchReturnInfo(t *testing.T) {
	testCases := []struct {
		name              string
		workflowID        string
		workflowDirectory string
	}{
		{
			name:              "GitHub",
			workflowID:        "dispatch.yml",
			workflowDirectory: ".github/workflows",
		},
		{
			name:              "Gitea",
			workflowID:        "test.yml",
			workflowDirectory: ".gitea/workflows",
		},
		{
			name:              "Forgejo",
			workflowID:        "build.yml",
			workflowDirectory: ".forgejo/workflows",
		},
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
				token := getUserToken(t, user2.LowerName, auth_model.AccessTokenScopeWriteRepository)

				// create the repo
				repo, _, f := tests.CreateDeclarativeRepo(t, user2, "api-repo-workflow-dispatch",
					[]unit_model.Type{unit_model.TypeActions}, nil,
					[]*files_service.ChangeRepoFile{
						{
							Operation: "create",
							TreePath:  fmt.Sprintf("%s/%s", testCase.workflowDirectory, testCase.workflowID),
							ContentReader: strings.NewReader(`name: WD
on: [workflow-dispatch]
jobs:
  t1:
    runs-on: docker
    steps:
      - run: echo "test 1"
  t2:
    runs-on: docker
    steps:
      - run: echo "test 2"
`,
							),
						},
					},
				)
				defer f()

				req := NewRequestWithJSON(
					t,
					http.MethodPost,
					fmt.Sprintf(
						"/api/v1/repos/%s/%s/actions/workflows/%s/dispatches",
						repo.OwnerName, repo.Name, testCase.workflowID,
					),
					&api.DispatchWorkflowOption{
						Ref:           repo.DefaultBranch,
						ReturnRunInfo: true,
					},
				)
				req.AddTokenAuth(token)

				res := MakeRequest(t, req, http.StatusCreated)
				run := new(api.DispatchWorkflowRun)
				DecodeJSON(t, res, run)

				assert.NotZero(t, run.ID)
				assert.NotZero(t, run.RunNumber)
				assert.Len(t, run.Jobs, 2)

				actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
				assert.Equal(t, "WD", actionRun.Title)
				assert.Equal(t, repo.ID, actionRun.RepoID)
				assert.Equal(t, repo.OwnerID, actionRun.OwnerID)
				assert.Equal(t, testCase.workflowID, actionRun.WorkflowID)
				assert.Equal(t, testCase.workflowDirectory, actionRun.WorkflowDirectory)
				assert.Equal(t, user2.ID, actionRun.TriggerUserID)
				assert.Zero(t, actionRun.ScheduleID)
				assert.Equal(t, "refs/heads/main", actionRun.Ref)
				assert.Equal(t, webhook.HookEventType("workflow_dispatch"), actionRun.Event)
				assert.Equal(t, "workflow_dispatch", actionRun.TriggerEvent)

				req = NewRequestWithJSON(
					t,
					http.MethodPost,
					fmt.Sprintf(
						"/api/v1/repos/%s/%s/actions/workflows/%s/dispatches",
						repo.OwnerName, repo.Name, testCase.workflowID,
					),
					&api.DispatchWorkflowOption{
						Ref:           repo.DefaultBranch,
						ReturnRunInfo: false,
					},
				)
				req.AddTokenAuth(token)
				res = MakeRequest(t, req, http.StatusNoContent)
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				assert.Empty(t, body) // 204 No Content doesn't support a body, so should be empty
			})
		}
	})
}

func TestActionsAPIGetListActionRun(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	var (
		runIDs = []int64{892, 893, 894}
		dbRuns = make(map[int64]*actions_model.ActionRun, 3)
	)

	for _, id := range runIDs {
		dbRuns[id] = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: id})
	}

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: dbRuns[runIDs[0]].RepoID})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	token := getUserToken(t, user.LowerName, auth_model.AccessTokenScopeWriteRepository)

	testqueries := []struct {
		name        string
		query       string
		expectedIDs []int64
	}{
		{
			name:        "No query parameters",
			query:       "",
			expectedIDs: runIDs,
		},
		{
			name:        "Search for workflow_dispatch events",
			query:       "?event=workflow_dispatch",
			expectedIDs: []int64{894},
		},
		{
			name:        "Search for multiple events",
			query:       "?event=workflow_dispatch&event=push",
			expectedIDs: []int64{892, 894},
		},
		{
			name:        "Search for failed status",
			query:       "?status=failure",
			expectedIDs: []int64{893},
		},
		{
			name:        "Search for multiple statuses",
			query:       "?status=failure&status=running",
			expectedIDs: []int64{893, 894},
		},
		{
			name:        "Search for num_nr",
			query:       "?run_number=1",
			expectedIDs: []int64{892},
		},
		{
			name:        "Search for sha",
			query:       "?head_sha=97f29ee599c373c729132a5c46a046978311e0ee",
			expectedIDs: []int64{892, 894},
		},
		{
			name:        "Search for Git reference",
			query:       "?ref=refs/heads/main",
			expectedIDs: []int64{892, 894},
		},
	}

	for _, tt := range testqueries {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(t, http.MethodGet,
				fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs%s",
					repo.OwnerName, repo.Name, tt.query,
				),
			)
			req.AddTokenAuth(token)

			res := MakeRequest(t, req, http.StatusOK)
			apiRuns := new(api.ListActionRunResponse)
			DecodeJSON(t, res, apiRuns)

			assert.Equal(t, int64(len(tt.expectedIDs)), apiRuns.TotalCount)
			assert.Len(t, apiRuns.Entries, len(tt.expectedIDs))

			resultIDs := make([]int64, apiRuns.TotalCount)
			for i, run := range apiRuns.Entries {
				resultIDs[i] = run.ID
			}

			assert.ElementsMatch(t, tt.expectedIDs, resultIDs)
		})
	}
}

func TestActionsAPIGetActionRun(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 63})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	token := getUserToken(t, user.LowerName, auth_model.AccessTokenScopeWriteRepository)

	testqueries := []struct {
		name           string
		runID          int64
		expectedStatus int
	}{
		{
			name:           "existing return ok",
			runID:          892,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non existing run",
			runID:          9876543210, // I hope this run will not exists, else just change it to another.
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "existing run but wrong repo should not be found",
			runID:          891,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range testqueries {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(t, http.MethodGet,
				fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d",
					repo.OwnerName, repo.Name, tt.runID,
				),
			)
			req.AddTokenAuth(token)

			res := MakeRequest(t, req, tt.expectedStatus)

			// Only interested in the data if 200 OK
			if tt.expectedStatus != http.StatusOK {
				return
			}

			dbRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: tt.runID})
			apiRun := new(api.ActionRun)
			DecodeJSON(t, res, apiRun)

			assert.Equal(t, dbRun.Index, apiRun.Index)
			assert.Equal(t, dbRun.Status.String(), apiRun.Status)
			assert.Equal(t, dbRun.CommitSHA, apiRun.CommitSHA)
			assert.Equal(t, dbRun.TriggerUserID, apiRun.TriggerUser.ID)
		})
	}
}

func TestAPIRepoActionsRunnerRegistrationTokenOperations(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestAPIRepoActionsRunnerRegistrationTokenOperations")()
	require.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user2.Name)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	t.Run("GetRegistrationToken", func(t *testing.T) {
		request := NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners/registration-token")
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusOK)

		var registrationToken shared.RegistrationToken
		DecodeJSON(t, response, &registrationToken)

		expected := shared.RegistrationToken{Token: "BzcgyhjWhLeKGA4ihJIigeRDrcxrFESd0yizEpb7xZJ"}

		assert.Equal(t, expected, registrationToken)
	})
}

func TestAPIRepoActionsRunnerOperations(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestAPIRepoActionsRunnerOperations")()
	require.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user2.Name)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	runnerOne := &api.ActionRunner{
		ID:          899251,
		UUID:        "a3297f3a-ba5c-4a0f-878e-6cc8b8ac79ec",
		Name:        "runner-1-repository",
		Version:     "dev",
		OwnerID:     0,
		RepoID:      62,
		Description: "A superb runner",
		Labels:      []string{"debian", "gpu"},
		Status:      "offline",
	}
	runnerTwo := &api.ActionRunner{
		ID:          899252,
		UUID:        "6d2d13ef-b19f-47a8-85ad-e82e51f606c5",
		Name:        "runner-2-user",
		Version:     "11.3.1",
		OwnerID:     1,
		RepoID:      0,
		Description: "A splendid runner",
		Labels:      []string{"docker"},
		Status:      "offline",
	}
	runnerThree := &api.ActionRunner{
		ID:          899253,
		UUID:        "0a7e5e05-2da4-44d5-a72a-615da120cef6",
		Name:        "runner-3-repository",
		Version:     "11.3.1",
		OwnerID:     0,
		RepoID:      62,
		Description: "Another fine runner",
		Labels:      []string{"fedora"},
		Status:      "offline",
	}
	runnerFour := &api.ActionRunner{
		ID:          899254,
		UUID:        "6456ac1f-70ec-4e8f-9ab7-bf117ee23d47",
		Name:        "runner-4-global",
		Version:     "11.3.1",
		OwnerID:     0,
		RepoID:      0,
		Description: "",
		Labels:      []string{},
		Status:      "offline",
	}
	runnerFive := &api.ActionRunner{
		ID:          899255,
		UUID:        "96639646-67b2-4bcb-9142-fde1ab8498cf",
		Name:        "runner-5-repository-ephemeral",
		Version:     "1.0.0",
		OwnerID:     0,
		RepoID:      62,
		Description: "An ephemeral runner",
		Labels:      []string{"ephemeral-label"},
		Status:      "offline",
		Ephemeral:   true,
	}

	t.Run("Get runners", func(t *testing.T) {
		request := NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners")
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusOK)

		assert.Equal(t, "3", response.Header().Get("X-Total-Count"))

		var runners []*api.ActionRunner
		DecodeJSON(t, response, &runners)

		assert.ElementsMatch(t, []*api.ActionRunner{runnerOne, runnerThree, runnerFive}, runners)
	})

	t.Run("Get runners paginated", func(t *testing.T) {
		request := NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners?page=1&limit=1")
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusOK)

		var runners []*api.ActionRunner
		DecodeJSON(t, response, &runners)

		assert.NotEmpty(t, response.Header().Get("Link"))
		assert.NotEmpty(t, response.Header().Get("X-Total-Count"))
		assert.Len(t, runners, 1)
	})

	t.Run("Get visible runners", func(t *testing.T) {
		request := NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners?visible=true")
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusOK)

		assert.NotEmpty(t, response.Header().Get("X-Total-Count"))

		var runners []*api.ActionRunner
		DecodeJSON(t, response, &runners)

		// There are more runners in the result that originate from the global fixtures. The test ignores them to limit
		// the impact of unrelated changes.
		assert.Contains(t, runners, runnerOne)
		assert.NotContains(t, runners, runnerTwo)
		assert.Contains(t, runners, runnerThree)
		assert.Contains(t, runners, runnerFour)
		assert.Contains(t, runners, runnerFive)
	})

	t.Run("Get runner", func(t *testing.T) {
		request := NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners/899251")
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusOK)

		var runner *api.ActionRunner
		DecodeJSON(t, response, &runner)

		assert.Equal(t, runnerOne, runner)

		// Runner of instance is visible
		request = NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners/899254")
		request.AddTokenAuth(readToken)
		response = MakeRequest(t, request, http.StatusOK)

		DecodeJSON(t, response, &runner)

		assert.Equal(t, runnerFour, runner)

		// Runner of user that does not own the repository is invisible
		request = NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners/899252")
		request.AddTokenAuth(readToken)
		MakeRequest(t, request, http.StatusNotFound)
	})

	t.Run("Get ephemeral runner", func(t *testing.T) {
		request := NewRequest(t, "GET", "/api/v1/repos/user2/test_workflows/actions/runners/899255")
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusOK)

		var runner *api.ActionRunner
		DecodeJSON(t, response, &runner)

		assert.Equal(t, runnerFive, runner)
	})

	t.Run("Delete runner", func(t *testing.T) {
		url := "/api/v1/repos/user2/test_workflows/actions/runners/899253"

		request := NewRequest(t, "GET", url)
		request.AddTokenAuth(readToken)
		MakeRequest(t, request, http.StatusOK)

		deleteRequest := NewRequest(t, "DELETE", url)
		deleteRequest.AddTokenAuth(writeToken)
		MakeRequest(t, deleteRequest, http.StatusNoContent)

		request = NewRequest(t, "GET", url)
		request.AddTokenAuth(readToken)
		MakeRequest(t, request, http.StatusNotFound)
	})

	t.Run("Register runner", func(t *testing.T) {
		options := api.RegisterRunnerOptions{Name: "api-runner", Description: "Some description"}

		requestURL := fmt.Sprintf("/api/v1/repos/%s/%s/actions/runners", repo1.OwnerName, repo1.Name)
		request := NewRequestWithJSON(t, "POST", requestURL, options)
		request.AddTokenAuth(writeToken)
		response := MakeRequest(t, request, http.StatusCreated)

		var registerRunnerResponse *api.RegisterRunnerResponse
		DecodeJSON(t, response, &registerRunnerResponse)

		assert.NotNil(t, registerRunnerResponse)
		assert.Positive(t, registerRunnerResponse.ID)
		assert.Equal(t, gouuid.Version(4), gouuid.MustParse(registerRunnerResponse.UUID).Version())
		assert.Regexp(t, "(?i)^[0-9a-f]{40}$", registerRunnerResponse.Token)

		registeredRunner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{UUID: registerRunnerResponse.UUID})
		assert.Equal(t, registerRunnerResponse.ID, registeredRunner.ID)
		assert.Equal(t, registerRunnerResponse.UUID, registeredRunner.UUID)
		assert.Zero(t, registeredRunner.OwnerID)
		assert.Equal(t, repo1.ID, registeredRunner.RepoID)
		assert.Equal(t, "api-runner", registeredRunner.Name)
		assert.Equal(t, "Some description", registeredRunner.Description)
		assert.Empty(t, registeredRunner.AgentLabels)
		assert.Empty(t, registeredRunner.Version)
		assert.NotEmpty(t, registeredRunner.TokenHash)
		assert.NotEmpty(t, registeredRunner.TokenSalt)
		assert.False(t, registeredRunner.Ephemeral)
	})

	t.Run("Register ephemeral runner", func(t *testing.T) {
		options := api.RegisterRunnerOptions{Name: "ephemeral-runner", Description: "Ephemeral runner", Ephemeral: true}

		requestURL := fmt.Sprintf("/api/v1/repos/%s/%s/actions/runners", repo1.OwnerName, repo1.Name)
		request := NewRequestWithJSON(t, "POST", requestURL, options)
		request.AddTokenAuth(writeToken)
		response := MakeRequest(t, request, http.StatusCreated)

		var registerRunnerResponse *api.RegisterRunnerResponse
		DecodeJSON(t, response, &registerRunnerResponse)

		registeredRunner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{UUID: registerRunnerResponse.UUID})
		assert.Equal(t, registerRunnerResponse.UUID, registeredRunner.UUID)
		assert.True(t, registeredRunner.Ephemeral)
	})

	t.Run("Runner registration does not update runner with identical name", func(t *testing.T) {
		options := api.RegisterRunnerOptions{Name: "api-runner"}

		requestURL := fmt.Sprintf("/api/v1/repos/%s/%s/actions/runners", repo1.OwnerName, repo1.Name)
		request := NewRequestWithJSON(t, "POST", requestURL, options)
		request.AddTokenAuth(writeToken)
		response := MakeRequest(t, request, http.StatusCreated)

		var registerRunnerResponse *api.RegisterRunnerResponse
		DecodeJSON(t, response, &registerRunnerResponse)

		secondRequest := NewRequestWithJSON(t, "POST", requestURL, options)
		secondRequest.AddTokenAuth(writeToken)
		secondResponse := MakeRequest(t, secondRequest, http.StatusCreated)

		var secondRegisterRunnerResponse *api.RegisterRunnerResponse
		DecodeJSON(t, secondResponse, &secondRegisterRunnerResponse)

		firstRunner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{UUID: registerRunnerResponse.UUID})
		secondRunner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{UUID: secondRegisterRunnerResponse.UUID})

		assert.NotEqual(t, firstRunner.ID, secondRunner.ID)
		assert.NotEqual(t, firstRunner.UUID, secondRunner.UUID)
	})

	t.Run("Runner registration requires write token for repository scope", func(t *testing.T) {
		options := api.RegisterRunnerOptions{Name: "api-runner"}

		requestURL := fmt.Sprintf("/api/v1/repos/%s/%s/actions/runners", repo1.OwnerName, repo1.Name)
		request := NewRequestWithJSON(t, "POST", requestURL, options)
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusForbidden)

		type errorResponse struct {
			Message string `json:"message"`
		}

		var errorMessage *errorResponse
		DecodeJSON(t, response, &errorMessage)

		assert.Equal(t, "token does not have at least one of required scope(s): [write:repository]", errorMessage.Message)
	})

	t.Run("Endpoints disabled if Actions disabled", func(t *testing.T) {
		repository, _, cleanUp := tests.CreateDeclarativeRepo(t, user2, "no-actions",
			[]unit_model.Type{unit_model.TypeCode, unit_model.TypeActions}, []unit_model.Type{}, nil)
		defer cleanUp()

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runners", repository.FullName())

		request := NewRequest(t, "GET", requestURL)
		request.AddTokenAuth(readToken)
		MakeRequest(t, request, http.StatusOK)

		enabledUnits := []repo_model.RepoUnit{{RepoID: repository.ID, Type: unit_model.TypeCode}}
		disabledUnits := []unit_model.Type{unit_model.TypeActions}
		err := repo_service.UpdateRepositoryUnits(db.DefaultContext, repository, enabledUnits, disabledUnits)
		require.NoError(t, err)

		request = NewRequest(t, "GET", requestURL)
		request.AddTokenAuth(readToken)
		MakeRequest(t, request, http.StatusNotFound)
	})
}

func TestActionsAPIDeleteActionRun(t *testing.T) {
	t.Run("Run removed", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIDeleteActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34901})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID})
		unittest.AssertCount(t, &actions_model.ActionArtifact{RunID: run.ID}, 2)
		runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: 41601})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d", repo1.FullName(), run.ID)
		request := NewRequest(t, "DELETE", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNoContent)

		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: run.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRunJob{ID: job.ID})
		unittest.AssertCount(t, &actions_model.ActionArtifact{
			RunID:  run.ID,
			Status: int64(actions_model.ArtifactStatusPendingDeletion),
		}, 2)
		unittest.AssertNotExistsBean(t, &actions_model.ActionRunner{ID: runner.ID})
	})

	t.Run("Error if run has not completed", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIDeleteActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34902})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d", repo1.FullName(), run.ID)
		request := NewRequest(t, "DELETE", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusInternalServerError)

		// Verify that the run still exists.
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	})

	t.Run("Not found if run does not belong to repository", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIDeleteActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34901})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d", repo62.FullName(), run.ID)
		request := NewRequest(t, "DELETE", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)

		// Verify that the run still exists.
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	})

	t.Run("No found if run does not exist", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: 260871})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/260871", repo1.FullName())
		request := NewRequest(t, "DELETE", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)
	})

	t.Run("Run removal requires write token", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIDeleteActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 34901})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d", repo1.FullName(), run.ID)
		request := NewRequest(t, "DELETE", requestURL)
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusForbidden)

		type errorResponse struct {
			Message string `json:"message"`
		}

		var errorMessage *errorResponse
		DecodeJSON(t, response, &errorMessage)

		assert.Equal(t, "token does not have at least one of required scope(s): [write:repository]", errorMessage.Message)

		// Verify that the run still exists.
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	})
}

func TestActionsAPICancelActionRun(t *testing.T) {
	t.Run("Run cancelled", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPICancelActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35011})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48011, RunID: run.ID})
		assert.False(t, run.Status.IsDone())
		assert.False(t, job.Status.IsDone())

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/cancel", repo1.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNoContent)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusCancelled, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusCancelled, job.Status)
	})

	t.Run("Already finished run is left unchanged", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPICancelActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35012})
		require.Equal(t, actions_model.StatusSuccess, run.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/cancel", repo1.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNoContent)

		// The finished run keeps its status.
		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusSuccess, run.Status)
	})

	t.Run("Not found if run does not belong to repository", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPICancelActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35011})
		assert.Equal(t, actions_model.StatusRunning, run.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/cancel", repo62.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)

		// Verify that the run was not cancelled.
		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusRunning, run.Status)
	})

	t.Run("Not found if run does not exist", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: 260871})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/260871/cancel", repo1.FullName())
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)
	})

	t.Run("Run cancellation requires write token", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPICancelActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35011})
		assert.Equal(t, actions_model.StatusRunning, run.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/cancel", repo1.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusForbidden)

		type errorResponse struct {
			Message string `json:"message"`
		}

		var errorMessage *errorResponse
		DecodeJSON(t, response, &errorMessage)

		assert.Equal(t, "token does not have at least one of required scope(s): [write:repository]", errorMessage.Message)

		// Verify that the run was not cancelled.
		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusRunning, run.Status)
	})
}

func TestActionsAPIRerunActionRun(t *testing.T) {
	t.Run("Run rerun", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35021})
		failedJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48021, RunID: run.ID})
		successfulJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48022, RunID: run.ID})
		require.Equal(t, actions_model.StatusFailure, run.Status)
		require.Equal(t, actions_model.StatusFailure, failedJob.Status)
		require.Equal(t, actions_model.StatusSuccess, successfulJob.Status)
		require.EqualValues(t, 1, successfulJob.Attempt)
		require.EqualValues(t, 1, failedJob.Attempt)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/rerun", repo1.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNoContent)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusWaiting, run.Status)
		failedJob = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: failedJob.ID})
		require.EqualValues(t, 48021, failedJob.ID)
		assert.Equal(t, actions_model.StatusWaiting, failedJob.Status)
		assert.EqualValues(t, 2, failedJob.Attempt)

		// Verify that both jobs were rerun
		successfulJob = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: successfulJob.ID})
		require.EqualValues(t, 48022, successfulJob.ID)
		assert.Equal(t, actions_model.StatusWaiting, successfulJob.Status)
		assert.EqualValues(t, 2, successfulJob.Attempt)
	})

	t.Run("Still running run cannot be rerun", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35022})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48023, RunID: run.ID})
		require.Equal(t, actions_model.StatusRunning, run.Status)
		require.Equal(t, actions_model.StatusRunning, job.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/rerun", repo1.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusBadRequest)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusRunning, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusRunning, job.Status)
		assert.EqualValues(t, 1, job.Attempt)
	})

	t.Run("Not found if run does not belong to repository", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35021})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48021, RunID: run.ID})
		require.Equal(t, actions_model.StatusFailure, run.Status)
		require.Equal(t, actions_model.StatusFailure, job.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/rerun", repo62.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusFailure, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusFailure, job.Status)
		assert.EqualValues(t, 1, job.Attempt)
	})

	t.Run("Not found if run does not exist", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: 260871})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/260871/rerun", repo1.FullName())
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)
	})

	t.Run("Run rerun requires write token", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionRun")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35021})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48021, RunID: run.ID})
		require.Equal(t, actions_model.StatusFailure, run.Status)
		require.Equal(t, actions_model.StatusFailure, job.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/rerun", repo1.FullName(), run.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusForbidden)

		type errorResponse struct {
			Message string `json:"message"`
		}

		var errorMessage *errorResponse
		DecodeJSON(t, response, &errorMessage)

		assert.Equal(t, "token does not have at least one of required scope(s): [write:repository]", errorMessage.Message)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusFailure, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusFailure, job.Status)
		assert.EqualValues(t, 1, job.Attempt)
	})
}

func TestActionsAPIRerunActionJob(t *testing.T) {
	t.Run("Job rerun", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionJob")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35031})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48031, RunID: run.ID})
		independentJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48032, RunID: run.ID})
		require.Equal(t, actions_model.StatusFailure, run.Status)
		require.Equal(t, actions_model.StatusFailure, job.Status)
		require.Equal(t, actions_model.StatusSuccess, independentJob.Status)
		require.EqualValues(t, 1, job.Attempt)
		require.EqualValues(t, 1, independentJob.Attempt)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/jobs/%d/rerun", repo1.FullName(), job.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNoContent)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusWaiting, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.EqualValues(t, 48031, job.ID)
		assert.Equal(t, actions_model.StatusWaiting, job.Status)
		assert.EqualValues(t, 2, job.Attempt)

		// Verify that the independent job was not rerun
		independentJob = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: independentJob.ID})
		require.EqualValues(t, 48032, independentJob.ID)
		assert.Equal(t, actions_model.StatusSuccess, independentJob.Status)
		assert.EqualValues(t, 1, independentJob.Attempt)
	})

	t.Run("Still running job cannot be rerun", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionJob")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35032})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48033, RunID: run.ID})
		require.Equal(t, actions_model.StatusRunning, run.Status)
		require.Equal(t, actions_model.StatusRunning, job.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/jobs/%d/rerun", repo1.FullName(), job.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusBadRequest)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusRunning, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusRunning, job.Status)
		assert.EqualValues(t, 1, job.Attempt)
	})

	t.Run("Not found if job does not belong to repository", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionJob")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo62 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35031})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48031, RunID: run.ID})
		require.Equal(t, actions_model.StatusFailure, run.Status)
		require.Equal(t, actions_model.StatusFailure, job.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/jobs/%d/rerun", repo62.FullName(), job.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusFailure, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusFailure, job.Status)
		assert.EqualValues(t, 1, job.Attempt)
	})

	t.Run("Not found if job does not exist", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		unittest.AssertNotExistsBean(t, &actions_model.ActionRunJob{ID: 260871})

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/jobs/260871/rerun", repo1.FullName())
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(writeToken)
		MakeRequest(t, request, http.StatusNotFound)
	})

	t.Run("Job rerun requires write token", func(t *testing.T) {
		defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionsAPIRerunActionJob")()
		defer tests.PrepareTestEnv(t)()

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: user2.ID})
		session := loginUser(t, user2.Name)
		readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 35031})
		job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 48031, RunID: run.ID})
		require.Equal(t, actions_model.StatusFailure, run.Status)
		require.Equal(t, actions_model.StatusFailure, job.Status)

		requestURL := fmt.Sprintf("/api/v1/repos/%s/actions/jobs/%d/rerun", repo1.FullName(), job.ID)
		request := NewRequest(t, "POST", requestURL)
		request.AddTokenAuth(readToken)
		response := MakeRequest(t, request, http.StatusForbidden)

		type errorResponse struct {
			Message string `json:"message"`
		}

		var errorMessage *errorResponse
		DecodeJSON(t, response, &errorMessage)

		assert.Equal(t, "token does not have at least one of required scope(s): [write:repository]", errorMessage.Message)

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.Equal(t, actions_model.StatusFailure, run.Status)
		job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, actions_model.StatusFailure, job.Status)
		assert.EqualValues(t, 1, job.Attempt)
	})
}

func TestActionsAPIListActionRunJobs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Jobs", func(t *testing.T) {
		for _, setup := range []struct {
			runID, repoID int64
		}{
			{793, 4},
			{895, 4},
		} {
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: setup.repoID})
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
			token := getUserToken(t, user.LowerName, auth_model.AccessTokenScopeReadRepository)
			req := NewRequest(t, http.MethodGet,
				fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/jobs",
					repo.OwnerName, repo.Name, setup.runID,
				),
			).AddTokenAuth(token)
			res := MakeRequest(t, req, http.StatusOK)
			var jobList []*api.ActionRunJob
			DecodeJSON(t, res, &jobList)

			correctJobList, err := actions_model.GetRunJobsByRunID(context.Background(), setup.runID)
			require.NoError(t, err, "GetRunJobsByRunID")
			assert.Len(t, jobList, len(correctJobList))

			for i := range jobList {
				expected := correctJobList[i]
				actual := jobList[i]
				assert.Equal(t, expected.ID, actual.ID)
				assert.Equal(t, expected.Attempt, actual.Attempt)
				assert.Equal(t, expected.Handle, actual.Handle)
				assert.Equal(t, expected.RepoID, actual.RepoID)
				assert.Equal(t, expected.OwnerID, actual.OwnerID)
				assert.Equal(t, expected.Name, actual.Name)
				assert.Equal(t, util.ConvertSlice[actions_model.LocalJobIdentifier, string](expected.Needs), actual.Needs)
				assert.Equal(t, expected.RunsOn, actual.RunsOn)
				assert.Equal(t, expected.TaskID, actual.TaskID)
				assert.Equal(t, expected.Status.String(), actual.Status)

				if expected.ID == 195 {
					assert.Equal(t, &api.ActionRunJob{
						ID:      195,
						RunID:   793,
						Attempt: 1,
						Handle:  "",
						RepoID:  4,
						OwnerID: 1,
						Name:    "job1 (2)",
						Needs:   nil,
						RunsOn:  nil,
						TaskID:  50,
						Status:  "success",
					}, actual)
				} else if expected.ID == 197 {
					assert.Equal(t, &api.ActionRunJob{
						ID:      197,
						RunID:   895,
						Attempt: 0,
						Handle:  "",
						RepoID:  4,
						OwnerID: 1,
						Name:    "job1 (1)",
						Needs:   nil,
						RunsOn:  []string{"postmarketOS"},
						TaskID:  54,
						Status:  "failure",
					}, actual)
				}
			}
		}
	})

	repoID := int64(4)
	runID := int64(793)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	token := getUserToken(t, user.LowerName, auth_model.AccessTokenScopeReadRepository)

	t.Run("Wrong Run ID", func(t *testing.T) {
		req := NewRequest(t, http.MethodGet,
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/jobs",
				repo.OwnerName, repo.Name, runID+9999,
			),
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Wrong Repo Name", func(t *testing.T) {
		req := NewRequest(t, http.MethodGet,
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/jobs",
				repo.OwnerName, repo.Name+"_wrong_repo", runID,
			),
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Wrong Owner", func(t *testing.T) {
		req := NewRequest(t, http.MethodGet,
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/jobs",
				repo.OwnerName+"_wrong_owner", repo.Name, runID,
			),
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
