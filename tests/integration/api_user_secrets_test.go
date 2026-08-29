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
	secret_model "forgejo.org/models/secret"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	api "forgejo.org/modules/structs"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIUserSecrets(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	t.Run("List", func(t *testing.T) {
		listURL := "/api/v1/user/actions/secrets"
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
		expectedValues := []string{"FIRST", "LAST", "SEC2"}
		for i := range expectedValues {
			assert.Equal(t, expectedValues[i], secrets[i].Name)
		}
		assert.NotContains(t, res.Body.String(), "a secret to create")

		readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
		req = NewRequest(t, "GET", listURL).AddTokenAuth(readToken)
		res = MakeRequest(t, req, http.StatusOK)
		secrets = []*api.Secret{}
		DecodeJSON(t, res, &secrets)
		assert.Len(t, secrets, 3)
		for i := range expectedValues {
			assert.Equal(t, expectedValues[i], secrets[i].Name)
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
			url := fmt.Sprintf("/api/v1/user/actions/secrets/%s", c.Name)
			req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{Data: "  \r\ndàtä\t  "})
			req.AddTokenAuth(token)
			MakeRequest(t, req, c.ExpectedStatus)

			if c.ExpectedStatus < 300 {
				secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{OwnerID: user.ID, Name: strings.ToUpper(c.Name)})
				data, err := secret.GetDecryptedData()
				require.NoError(t, err)
				assert.Equal(t, "  \ndàtä\t  ", data)
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		name := "update_user_secret_and_test_data"
		url := fmt.Sprintf("/api/v1/user/actions/secrets/%s", name)

		req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{Data: "initial"})
		req.AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{Data: "  \r\nchängéd\t  "})
		req.AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{Name: strings.ToUpper(name)})
		data, err := secret.GetDecryptedData()
		require.NoError(t, err)
		assert.Equal(t, "  \nchängéd\t  ", data)
	})

	t.Run("Delete", func(t *testing.T) {
		name := "delete_secret"
		url := fmt.Sprintf("/api/v1/user/actions/secrets/%s", name)

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
		secret := secret_model.Secret{OwnerID: user.ID, RepoID: 0, Name: "FORGEJO_FORBIDDEN"}
		err := db.Insert(t.Context(), secret)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/user/actions/secrets/%s", secret.Name)

		req := NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
