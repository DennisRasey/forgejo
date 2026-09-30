// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	secret_model "forgejo.org/models/secret"
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

func TestAPIRepoSecrets(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("List", func(t *testing.T) {
		listURL := fmt.Sprintf("/api/v1/repos/%s/actions/secrets", repo.FullName())
		req := NewRequest(t, "GET", listURL).AddTokenAuth(token)
		res := MakeRequest(t, req, http.StatusOK)
		secrets := []*api.Secret{}
		DecodeJSON(t, res, &secrets)
		assert.Empty(t, secrets)

		createData := api.CreateOrUpdateSecretOption{Data: "a secret to create"}
		req = NewRequestWithJSON(t, "PUT", listURL+"/first", createData).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "PUT", listURL+"/sec2", createData).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "PUT", listURL+"/last", createData).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "GET", listURL).AddTokenAuth(token)
		res = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, res, &secrets)
		assert.Len(t, secrets, 3)
		expectedValues := []string{"FIRST", "SEC2", "LAST"}
		for _, secret := range secrets {
			assert.Contains(t, expectedValues, secret.Name)
		}
	})

	t.Run("Create", func(t *testing.T) {
		cases := []struct {
			Name           string
			ExpectedStatus int
		}{
			{
				Name:           "",
				ExpectedStatus: http.StatusMethodNotAllowed,
			},
			{
				Name:           "-",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "_",
				ExpectedStatus: http.StatusCreated,
			},
			{
				Name:           "ci",
				ExpectedStatus: http.StatusCreated,
			},
			{
				Name:           "secret",
				ExpectedStatus: http.StatusCreated,
			},
			{
				Name:           "2secret",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "FORGEJO_secret",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "GITEA_secret",
				ExpectedStatus: http.StatusBadRequest,
			},
			{
				Name:           "GITHUB_secret",
				ExpectedStatus: http.StatusBadRequest,
			},
		}

		for _, c := range cases {
			url := fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), c.Name)
			req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
				Data: "   \r\ndàtä\t   \r\n  ",
			})
			req.AddTokenAuth(token)
			MakeRequest(t, req, c.ExpectedStatus)

			if c.ExpectedStatus < 300 {
				secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{RepoID: repo.ID, Name: strings.ToUpper(c.Name)})

				assert.Equal(t, strings.ToUpper(c.Name), secret.Name)

				value, err := secret.GetDecryptedData()
				require.NoError(t, err)
				assert.Equal(t, "   \ndàtä\t   \n  ", value)
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		name := "update_repo_secret_and_test_data"
		url := fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), name)

		req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{Data: "initial"})
		req.AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{Data: "  \r\nchànged data\t\r\n "})
		req.AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{Name: strings.ToUpper(name)})
		data, err := secret.GetDecryptedData()
		require.NoError(t, err)
		assert.Equal(t, "  \nchànged data\t\n ", data)
	})

	t.Run("Delete", func(t *testing.T) {
		name := "delete_secret"
		url := fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), name)

		req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
			Data: "initial",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Delete with forbidden names", func(t *testing.T) {
		secret := secret_model.Secret{OwnerID: 0, RepoID: repo.ID, Name: "FORGEJO_FORBIDDEN"}
		err := db.Insert(t.Context(), secret)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/repos/%s/actions/secrets/%s", repo.FullName(), secret.Name)

		req := NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Endpoints disabled if Actions disabled", func(t *testing.T) {
		repo := forgery.CreateRepository(t, user, &forgery.CreateRepositoryOptions{Name: "no-actions"})
		forgery.EnableRepoUnits(t, repo, unit_model.TypeCode, unit_model.TypeActions)

		getURL := fmt.Sprintf("/api/v1/repos/%s/actions/secrets", repo.FullName())

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
	})
}
