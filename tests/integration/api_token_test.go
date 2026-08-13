// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/log"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/modules/util"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPICreateAndDeleteToken tests that token that was just created can be deleted
func TestAPICreateAndDeleteToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	newAccessToken := createAPIAccessTokenWithoutCleanUp(t, "test-key-1", user, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
	deleteAPIAccessToken(t, newAccessToken, user)

	newAccessToken = createAPIAccessTokenWithoutCleanUp(t, "test-key-2", user, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
	deleteAPIAccessToken(t, newAccessToken, user)
}

func TestAPIGetTokens(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	t.Run("GET w/ basic auth", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// with basic auth...
		req := NewRequest(t, "GET", "/api/v1/users/user2/tokens").
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		var accessTokens api.AccessTokenList
		DecodeJSON(t, resp, &accessTokens)

		require.Len(t, accessTokens, 1)
		at := accessTokens[0]
		assert.EqualValues(t, 3, at.ID)
		assert.Equal(t, "Token A", at.Name)
		assert.Equal(t, []string{""}, at.Scopes)
		assert.Empty(t, at.Token)
		assert.Equal(t, "69d28c91", at.TokenLastEight)
		assert.NotZero(t, at.Created)
		assert.Nil(t, at.Repositories) // not repo-specific access token - nil expected, not an empty array
	})

	t.Run("GET w/ token auth", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// ... or with a token.
		newAccessToken := createAPIAccessTokenWithoutCleanUp(t, "test-key-1", user, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
		req := NewRequest(t, "GET", "/api/v1/users/user2/tokens").
			AddTokenAuth(newAccessToken.Token)
		resp := MakeRequest(t, req, http.StatusOK)
		var accessTokens api.AccessTokenList
		DecodeJSON(t, resp, &accessTokens)
		deleteAPIAccessToken(t, newAccessToken, user)
	})

	t.Run("GET fine-grained token", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo2OnlyToken := createFineGrainedRepoAccessToken(t, "user2",
			[]auth_model.AccessTokenScope{auth_model.AccessTokenScopeReadUser},
			[]int64{2, 3},
		)

		req := NewRequest(t, "GET", "/api/v1/users/user2/tokens").
			AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		var accessTokens api.AccessTokenList
		DecodeJSON(t, resp, &accessTokens)

		found := false
		for _, token := range accessTokens {
			if strings.HasSuffix(repo2OnlyToken, token.TokenLastEight) {
				found = true
				assert.Len(t, token.Repositories, 2)

				repo2 := token.Repositories[0]
				assert.Equal(t, &api.RepositoryMeta{
					ID:       2,
					Name:     "repo2",
					Owner:    "user2",
					FullName: "user2/repo2",
				}, repo2)

				repo3 := token.Repositories[1]
				assert.Equal(t, &api.RepositoryMeta{
					ID:       3,
					Name:     "repo3",
					Owner:    "org3",
					FullName: "org3/repo3",
				}, repo3)
			}
		}
		assert.True(t, found)
	})
}

// TestAPIDeleteMissingToken ensures that error is thrown when token not found
func TestAPIDeleteMissingToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	req := NewRequestf(t, "DELETE", "/api/v1/users/user1/tokens/%d", unittest.NonexistentID).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusNotFound)
}

// TestAPIGetTokensPermission ensures that only the admin can get tokens from other users
func TestAPIGetTokensPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// admin can get tokens for other users
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	req := NewRequest(t, "GET", "/api/v1/users/user2/tokens").
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusOK)

	// non-admin can get tokens for himself
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	req = NewRequest(t, "GET", "/api/v1/users/user2/tokens").
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusOK)

	// non-admin can't get tokens for other users
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	req = NewRequest(t, "GET", "/api/v1/users/user2/tokens").
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusForbidden)
}

// TestAPIDeleteTokensPermission ensures that only the admin can delete tokens from other users
func TestAPIDeleteTokensPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// admin can delete tokens for other users
	createAPIAccessTokenWithoutCleanUp(t, "test-key-1", user2, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
	req := NewRequest(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-1").
		AddBasicAuth(admin.Name)
	MakeRequest(t, req, http.StatusNoContent)

	// non-admin can delete tokens for himself
	createAPIAccessTokenWithoutCleanUp(t, "test-key-2", user2, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
	req = NewRequest(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-2").
		AddBasicAuth(user2.Name)
	MakeRequest(t, req, http.StatusNoContent)

	// non-admin can't delete tokens for other users
	createAPIAccessTokenWithoutCleanUp(t, "test-key-3", user2, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
	req = NewRequest(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-3").
		AddBasicAuth(user4.Name)
	MakeRequest(t, req, http.StatusForbidden)
}

type permission struct {
	category auth_model.AccessTokenScopeCategory
	level    auth_model.AccessTokenScopeLevel
}

type requiredScopeTestCase struct {
	url                 string
	method              string
	requiredPermissions []permission
}

func (c *requiredScopeTestCase) Name() string {
	return fmt.Sprintf("%v %v", c.method, c.url)
}

// TestAPIDeniesPermissionBasedOnTokenScope tests that API routes forbid access
// when the correct token scope is not included.
func TestAPIDeniesPermissionBasedOnTokenScope(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// We'll assert that each endpoint, when fetched with a token with all
	// scopes *except* the ones specified, a forbidden status code is returned.
	//
	// This is to protect against endpoints having their access check copied
	// from other endpoints and not updated.
	//
	// Test cases are in alphabetical order by URL.
	testCases := []requiredScopeTestCase{
		{
			"/api/v1/admin/emails",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/admin/users",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/admin/users",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/admin/users/user2",
			"PATCH",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/admin/users/user2/orgs",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/admin/users/user2/orgs",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/admin/orgs",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/notifications",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryNotification,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/notifications",
			"PUT",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryNotification,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/org/org1/repos",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryOrganization,
					auth_model.Write,
				},
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/packages/user1/type/name/1",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryPackage,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/packages/user1/type/name/1",
			"DELETE",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryPackage,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1",
			"PATCH",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/branches",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/archive/foo",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/issues",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryIssue,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/media/foo",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/raw/foo",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/teams",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/teams/team1",
			"PUT",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/transfer",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		// Private repo
		{
			"/api/v1/repos/user2/repo2",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		// Private repo
		{
			"/api/v1/repos/user2/repo2",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user/emails",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user/emails",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/user/emails",
			"DELETE",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/user/applications/oauth2",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user/applications/oauth2",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/users/search",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		// Private user
		{
			"/api/v1/users/user31",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		// Private user
		{
			"/api/v1/users/user31/gpg_keys",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
	}

	// User needs to be admin so that we can verify that tokens without admin
	// scopes correctly deny access.
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.True(t, user.IsAdmin, "User needs to be admin")

	for _, testCase := range testCases {
		runTestCase(t, &testCase, user)
	}
}

// runTestCase Helper function to run a single test case.
func runTestCase(t *testing.T, testCase *requiredScopeTestCase, user *user_model.User) {
	t.Run(testCase.Name(), func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Create a token with all scopes NOT required by the endpoint.
		var unauthorizedScopes []auth_model.AccessTokenScope
		for _, category := range auth_model.AllAccessTokenScopeCategories {
			// For permissions, Write > Read > NoAccess.  So we need to
			// find the minimum required, and only grant permission up to but
			// not including the minimum required.
			minRequiredLevel := auth_model.Write
			categoryIsRequired := false
			for _, requiredPermission := range testCase.requiredPermissions {
				if requiredPermission.category != category {
					continue
				}
				categoryIsRequired = true
				if requiredPermission.level < minRequiredLevel {
					minRequiredLevel = requiredPermission.level
				}
			}
			unauthorizedLevel := auth_model.Write
			if categoryIsRequired {
				if minRequiredLevel == auth_model.Read {
					unauthorizedLevel = auth_model.NoAccess
				} else if minRequiredLevel == auth_model.Write {
					unauthorizedLevel = auth_model.Read
				} else {
					assert.FailNow(t, "Invalid test case", "Unknown access token scope level: %v", minRequiredLevel)
				}
			}

			if unauthorizedLevel == auth_model.NoAccess {
				continue
			}
			categoryUnauthorizedScopes := auth_model.GetRequiredScopes(
				unauthorizedLevel,
				category,
			)
			unauthorizedScopes = append(unauthorizedScopes, categoryUnauthorizedScopes...)
		}

		// Request the endpoint.  Verify that permission is denied.

		t.Run("Bearer", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			accessToken := createAPIAccessTokenWithoutCleanUp(t, "test-token", user, unauthorizedScopes)
			defer deleteAPIAccessToken(t, accessToken, user)

			req := NewRequest(t, testCase.method, testCase.url).
				AddTokenAuth(accessToken.Token)
			MakeRequest(t, req, http.StatusForbidden)
		})

		t.Run("Basic", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			oauth2Token := createOAuth2Token(t, loginUser(t, user.Name), unauthorizedScopes)
			defer unittest.AssertSuccessfulDelete(t, &auth_model.OAuth2Grant{ApplicationID: 2, UserID: user.ID})

			req := NewRequest(t, testCase.method, testCase.url)
			req.SetBasicAuth("x-oauth-basic", oauth2Token)
			MakeRequest(t, req, http.StatusForbidden)
		})
	})
}

// createAPIAccessTokenWithoutCleanUp Create an API access token and assert that
// creation succeeded.  The caller is responsible for deleting the token.
func createAPIAccessTokenWithoutCleanUp(t *testing.T, tokenName string, user *user_model.User, scopes []auth_model.AccessTokenScope) api.AccessToken {
	payload := map[string]any{
		"name":   tokenName,
		"scopes": scopes,
	}

	log.Debug("Requesting creation of token with scopes: %v", scopes)
	req := NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", payload).
		AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	var newAccessToken api.AccessToken
	DecodeJSON(t, resp, &newAccessToken)
	unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{
		ID:    newAccessToken.ID,
		Name:  newAccessToken.Name,
		Token: newAccessToken.Token,
		UID:   user.ID,
	})

	return newAccessToken
}

// deleteAPIAccessToken deletes an API access token and assert that deletion succeeded.
func deleteAPIAccessToken(t *testing.T, accessToken api.AccessToken, user *user_model.User) {
	req := NewRequestf(t, "DELETE", "/api/v1/users/"+user.LoginName+"/tokens/%d", accessToken.ID).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &auth_model.AccessToken{ID: accessToken.ID})
}

func createOAuth2Token(t *testing.T, session *TestSession, scopes []auth_model.AccessTokenScope) string {
	// Make a call to `/login/oauth/authorize` to get some session data.
	session.MakeRequest(t, NewRequest(t, "GET", "/login/oauth/authorize?client_id=ce5a1322-42a7-11ed-b878-0242ac120002&redirect_uri=b&response_type=code&code_challenge_method=plain&code_challenge=CODE&state=thestate"), http.StatusOK)

	var b strings.Builder
	switch len(scopes) {
	case 0:
		break
	case 1:
		b.WriteString(string(scopes[0]))
	default:
		b.WriteString(string(scopes[0]))
		for _, s := range scopes[1:] {
			b.WriteString(" ")
			b.WriteString(string(s))
		}
	}

	req := NewRequestWithValues(t, "POST", "/login/oauth/grant", map[string]string{
		"client_id":    "ce5a1322-42a7-11ed-b878-0242ac120002",
		"redirect_uri": "b",
		"state":        "thestate",
		"granted":      "true",
		"scope":        b.String(),
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)

	u, err := url.Parse(test.RedirectURL(resp))
	require.NoError(t, err)

	req = NewRequestWithValues(t, "POST", "/login/oauth/access_token", map[string]string{
		"client_id":     "ce5a1322-42a7-11ed-b878-0242ac120002",
		"code":          u.Query().Get("code"),
		"code_verifier": "CODE",
		"grant_type":    "authorization_code",
		"redirect_uri":  "b",
	})
	resp = MakeRequest(t, req, http.StatusOK)

	var respBody map[string]any
	DecodeJSON(t, resp, &respBody)

	return respBody["access_token"].(string)
}

func TestAPITokenCreation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user4")
	t.Run("Via API token", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

		req := NewRequestWithJSON(t, "POST", "/api/v1/users/user4/tokens", map[string]any{
			"name":   "new-new-token",
			"scopes": []auth_model.AccessTokenScope{auth_model.AccessTokenScopeWriteUser},
		})
		req.Request.Header.Set("Authorization", "basic "+base64.StdEncoding.EncodeToString([]byte("user4:"+token)))

		resp := MakeRequest(t, req, http.StatusUnauthorized)

		respMsg := map[string]any{}
		DecodeJSON(t, resp, &respMsg)

		assert.EqualValues(t, "auth method not allowed", respMsg["message"])
	})

	t.Run("Via OAuth2", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		accessToken := createOAuth2Token(t, session, nil)

		req := NewRequestWithJSON(t, "POST", "/api/v1/users/user4/tokens", map[string]any{
			"name":   "new-new-token",
			"scopes": []auth_model.AccessTokenScope{auth_model.AccessTokenScopeWriteUser},
		})
		req.SetBasicAuth("user4", accessToken)

		resp := MakeRequest(t, req, http.StatusUnauthorized)

		respMsg := map[string]any{}
		DecodeJSON(t, resp, &respMsg)

		assert.EqualValues(t, "auth method not allowed", respMsg["message"])
	})

	t.Run("Via password", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithJSON(t, "POST", "/api/v1/users/user4/tokens", map[string]any{
			"name":   "new-new-token",
			"scopes": []auth_model.AccessTokenScope{auth_model.AccessTokenScopeWriteUser},
		})
		req.AddBasicAuth("user4")

		resp := MakeRequest(t, req, http.StatusCreated)
		var token api.AccessToken
		DecodeJSON(t, resp, &token)
		assert.NotZero(t, token.Created)
	})

	t.Run("repo-specific", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		t.Run("valid", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequestWithJSON(t, "POST", "/api/v1/users/user2/tokens", &api.CreateAccessTokenOption{
				Name:   util.CryptoRandomString(util.RandomStringLow), // avoid false test failures from conflicting names
				Scopes: []string{string(auth_model.AccessTokenScopeReadRepository)},
				Repositories: []*api.RepoTargetOption{
					{
						Owner: "user2",
						Name:  "repo2",
					},
				},
			})
			req.AddBasicAuth("user2")

			resp := MakeRequest(t, req, http.StatusCreated)
			var token api.AccessToken
			DecodeJSON(t, resp, &token)
			assert.NotEmpty(t, token.Repositories)
		})

		t.Run("target other user's private repo", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequestWithJSON(t, "POST", "/api/v1/users/user2/tokens", &api.CreateAccessTokenOption{
				Name:   util.CryptoRandomString(util.RandomStringLow), // avoid unexpected test impact from conflicting names
				Scopes: []string{string(auth_model.AccessTokenScopeReadRepository)},
				Repositories: []*api.RepoTargetOption{
					{
						Owner: "user10",
						Name:  "repo7", // private repo owned by another user
					},
				},
			})
			req.AddBasicAuth("user2")
			MakeRequest(t, req, http.StatusBadRequest)
		})

		t.Run("target invalid repo", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequestWithJSON(t, "POST", "/api/v1/users/user2/tokens", &api.CreateAccessTokenOption{
				Name:   util.CryptoRandomString(util.RandomStringLow), // avoid unexpected test impact from conflicting names
				Scopes: []string{string(auth_model.AccessTokenScopeReadRepository)},
				Repositories: []*api.RepoTargetOption{
					{
						// doesn't exist:
						Owner: "user10000",
						Name:  "repo70000",
					},
				},
			})
			req.AddBasicAuth("user2")
			MakeRequest(t, req, http.StatusBadRequest)
		})

		t.Run("invalid scopes", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequestWithJSON(t, "POST", "/api/v1/users/user2/tokens", &api.CreateAccessTokenOption{
				Name:   util.CryptoRandomString(util.RandomStringLow), // avoid unexpected test impact from conflicting names
				Scopes: []string{string(auth_model.AccessTokenScopeReadAdmin)},
				Repositories: []*api.RepoTargetOption{
					{
						Owner: "user2",
						Name:  "repo2",
					},
				},
			})
			req.AddBasicAuth("user2")
			MakeRequest(t, req, http.StatusBadRequest)
		})

		t.Run("invalid zero repositories", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequestWithJSON(t, "POST", "/api/v1/users/user2/tokens", &api.CreateAccessTokenOption{
				Name:         util.CryptoRandomString(util.RandomStringLow), // avoid unexpected test impact from conflicting names
				Scopes:       []string{string(auth_model.AccessTokenScopeReadRepository)},
				Repositories: []*api.RepoTargetOption{}, // not nil, but not populated
			})
			req.AddBasicAuth("user2")
			MakeRequest(t, req, http.StatusBadRequest)
		})
	})
}

func TestAPITokenDelete(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequestWithJSON(t, "POST", "/api/v1/users/user2/tokens", &api.CreateAccessTokenOption{
		Name:   "delete-this-token",
		Scopes: []string{string(auth_model.AccessTokenScopeReadRepository)},
		Repositories: []*api.RepoTargetOption{
			{
				Owner: "user2",
				Name:  "repo2",
			},
		},
	})
	req.AddBasicAuth("user2")

	resp := MakeRequest(t, req, http.StatusCreated)
	var token api.AccessToken
	DecodeJSON(t, resp, &token)

	unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{ID: token.ID})

	req = NewRequestf(t, "DELETE", "/api/v1/users/user2/tokens/%d", token.ID)
	req.AddBasicAuth("user2")
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &auth_model.AccessToken{ID: token.ID})
}
