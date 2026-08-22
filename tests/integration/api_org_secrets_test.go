// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	org_model "forgejo.org/models/organization"
	secret_model "forgejo.org/models/secret"
	"forgejo.org/models/unittest"
	api "forgejo.org/modules/structs"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIOrgSecrets(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{Name: "org3"})
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("List", func(t *testing.T) {
		listURL := fmt.Sprintf("/api/v1/orgs/%s/actions/secrets", org.Name)
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
			url := fmt.Sprintf("/api/v1/orgs/%s/actions/secrets/%s", org.Name, c.Name)

			req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
				Data: "    \r\n\tdätå   \r\n",
			})
			req.AddTokenAuth(token)
			MakeRequest(t, req, c.ExpectedStatus)

			if c.ExpectedStatus < 300 {
				secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{OwnerID: org.ID, Name: strings.ToUpper(c.Name)})

				assert.Equal(t, strings.ToUpper(c.Name), secret.Name)

				value, err := secret.GetDecryptedData()
				require.NoError(t, err)
				assert.Equal(t, "    \n\tdätå   \n", value)
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		name := "update_org_secret_and_test_data"
		url := fmt.Sprintf("/api/v1/orgs/%s/actions/secrets/%s", org.Name, name)

		req := NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
			Data: "initial",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PUT", url, api.CreateOrUpdateSecretOption{
			Data: "\r\n    chåñgéd dätå\t   \r\n",
		})
		req.AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{Name: strings.ToUpper(name)})
		data, err := secret.GetDecryptedData()
		require.NoError(t, err)
		assert.Equal(t, "\n    chåñgéd dätå\t   \n", data)
	})

	t.Run("Delete", func(t *testing.T) {
		name := "delete_secret"
		url := fmt.Sprintf("/api/v1/orgs/%s/actions/secrets/%s", org.Name, name)

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
		secret := secret_model.Secret{OwnerID: org.ID, RepoID: 0, Name: "FORGEJO_FORBIDDEN"}
		err := db.Insert(t.Context(), secret)
		require.NoError(t, err)

		url := fmt.Sprintf("/api/v1/orgs/%s/actions/secrets/%s", org.Name, secret.Name)

		req := NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "DELETE", url).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
