// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	unit_model "forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	api "forgejo.org/modules/structs"
	repo_service "forgejo.org/services/repository"
	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoVariablesTestCreateRepositoryVariable(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	cases := []struct {
		Name           string
		ExpectedStatus int
	}{
		{
			Name:           "-",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "_",
			ExpectedStatus: http.StatusNoContent,
		},
		{
			Name:           "TEST_VAR",
			ExpectedStatus: http.StatusNoContent,
		},
		{
			Name:           "test_var",
			ExpectedStatus: http.StatusConflict,
		},
		{
			Name:           "ci",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "123var",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "var@test",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "forgejo_var",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "github_var",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "gitea_var",
			ExpectedStatus: http.StatusBadRequest,
		},
	}

	for _, c := range cases {
		url := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), c.Name)

		req := NewRequestWithJSON(t, "POST", url, api.CreateVariableOption{Value: "  \tvalüé\r\n" + c.Name + "  \r\n"})
		req.AddTokenAuth(token)
		MakeRequest(t, req, c.ExpectedStatus)

		if c.ExpectedStatus < 300 {
			req = NewRequest(t, "GET", url)
			req.AddTokenAuth(token)
			res := MakeRequest(t, req, http.StatusOK)

			variable := api.ActionVariable{}
			DecodeJSON(t, res, &variable)

			assert.Equal(t, variable.Name, c.Name)
			assert.Equal(t, variable.Data, "  \tvalüé\n"+c.Name+"  \n")
		}
	}
}

func TestAPIRepoVariablesUpdateRepositoryVariable(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	variableName := "test_update_var"
	url := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), variableName)
	req := NewRequestWithJSON(t, "POST", url, api.CreateVariableOption{Value: "initial_val"})
	req.AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	t.Run("Accepts only valid variable names", func(t *testing.T) {
		cases := []struct {
			Name           string
			UpdateName     string
			ExpectedStatus int
		}{
			{
				Name:           "not_found_var",
				ExpectedStatus: http.StatusNotFound,
			},
			{
				Name:           variableName,
				UpdateName:     "1invalid",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           variableName,
				UpdateName:     "invalid@name",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           variableName,
				UpdateName:     "ci",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           variableName,
				UpdateName:     "forgejo_foo",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           variableName,
				UpdateName:     "updated_var_name",
				ExpectedStatus: http.StatusNoContent,
			},
			{
				Name:           variableName,
				ExpectedStatus: http.StatusNotFound,
			},
			{
				Name:           "updated_var_name",
				ExpectedStatus: http.StatusNoContent,
			},
		}

		for _, c := range cases {
			url := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), c.Name)
			requestData := api.UpdateVariableOption{
				Name:  c.UpdateName,
				Value: "updated_val",
			}
			req := NewRequestWithJSON(t, "PUT", url, requestData)
			req.AddTokenAuth(token)
			MakeRequest(t, req, c.ExpectedStatus)
		}
	})

	t.Run("Retains special characters", func(t *testing.T) {
		variableName := "special_characters"
		url := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), variableName)

		req := NewRequestWithJSON(t, "POST", url, api.CreateVariableOption{Value: "initial_value"})
		req.AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		requestData := api.UpdateVariableOption{
			Value: "\r\n    \tüpdåtéd\r\n   \r\n",
		}
		req = NewRequestWithJSON(t, "PUT", url, requestData)
		req.AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", url)
		req.AddTokenAuth(token)
		res := MakeRequest(t, req, http.StatusOK)

		variable := api.ActionVariable{}
		DecodeJSON(t, res, &variable)

		assert.Equal(t, "SPECIAL_CHARACTERS", variable.Name)
		assert.Equal(t, "\n    \tüpdåtéd\n   \n", variable.Data)
	})
}

func TestAPIRepoVariablesDeleteRepositoryVariable(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	variable, err := actions_model.InsertVariable(t.Context(), 0, repo.ID, "FORGEJO_FORBIDDEN", "illegal")
	require.NoError(t, err)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	variableName := "test_delete_var"
	url := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), variableName)

	req := NewRequestWithJSON(t, "POST", url, api.CreateVariableOption{
		Value: "initial_val",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// deleting of forbidden names should still be possible
	url = fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), variable.Name)
	req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "DELETE", url).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIRepoVariablesGetSingleRepositoryVariable(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	name := "some_variable"
	value := "false"

	createURL := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), name)

	createRequest := NewRequestWithJSON(t, "POST", createURL, api.CreateVariableOption{Value: value})
	createRequest.AddTokenAuth(token)
	MakeRequest(t, createRequest, http.StatusNoContent)

	getURL := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), name)

	getRequest := NewRequest(t, "GET", getURL)
	getRequest.AddTokenAuth(token)
	getResponse := MakeRequest(t, getRequest, http.StatusOK)

	var actionVariable api.ActionVariable
	DecodeJSON(t, getResponse, &actionVariable)

	assert.NotNil(t, actionVariable)
	assert.Equal(t, int64(0), actionVariable.OwnerID)
	assert.Equal(t, repo.ID, actionVariable.RepoID)
	assert.Equal(t, "SOME_VARIABLE", actionVariable.Name)
	assert.Equal(t, value, actionVariable.Data)
}

func TestAPIRepoVariablesGetAllRepositoryVariables(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	variables := map[string]string{"second": "Dolor sit amet", "first": "Lorem ipsum"}
	for name, value := range variables {
		createURL := fmt.Sprintf("/api/v1/repos/%s/actions/variables/%s", repo.FullName(), name)

		createRequest := NewRequestWithJSON(t, "POST", createURL, api.CreateVariableOption{Value: value})
		createRequest.AddTokenAuth(token)

		MakeRequest(t, createRequest, http.StatusNoContent)
	}

	getURL := fmt.Sprintf("/api/v1/repos/%s/actions/variables", repo.FullName())

	getRequest := NewRequest(t, "GET", getURL)
	getRequest.AddTokenAuth(token)
	getResponse := MakeRequest(t, getRequest, http.StatusOK)

	var actionVariables []api.ActionVariable
	DecodeJSON(t, getResponse, &actionVariables)

	assert.Len(t, actionVariables, len(variables))

	assert.Equal(t, int64(0), actionVariables[0].OwnerID)
	assert.Equal(t, repo.ID, actionVariables[0].RepoID)
	assert.Equal(t, "FIRST", actionVariables[0].Name)
	assert.Equal(t, "Lorem ipsum", actionVariables[0].Data)

	assert.Equal(t, int64(0), actionVariables[1].OwnerID)
	assert.Equal(t, repo.ID, actionVariables[1].RepoID)
	assert.Equal(t, "SECOND", actionVariables[1].Name)
	assert.Equal(t, "Dolor sit amet", actionVariables[1].Data)
}

func TestAPIRepoVariablesEndpointsDisabledIfActionsDisabled(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	repo := forgery.CreateRepository(t, user2, &forgery.CreateRepositoryOptions{Name: "no-actions"})
	forgery.EnableRepoUnits(t, repo, unit_model.TypeCode, unit_model.TypeActions)

	getURL := fmt.Sprintf("/api/v1/repos/%s/actions/variables", repo.FullName())

	getRequest := NewRequest(t, "GET", getURL)
	getRequest.AddTokenAuth(token)
	MakeRequest(t, getRequest, http.StatusOK)

	enabledUnits := []repo_model.RepoUnit{{RepoID: repo.ID, Type: unit_model.TypeCode}}
	disabledUnits := []unit_model.Type{unit_model.TypeActions}
	err := repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, enabledUnits, disabledUnits)
	require.NoError(t, err)

	getRequest = NewRequest(t, "GET", getURL)
	getRequest.AddTokenAuth(token)
	MakeRequest(t, getRequest, http.StatusNotFound)
}
