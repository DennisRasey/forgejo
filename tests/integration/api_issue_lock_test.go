// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "forgejo.org/models/auth"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	api "forgejo.org/modules/structs"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPILockIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	require.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Lock the Issue
	urlAPI := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "PUT", urlAPI, &api.IssueLockOption{
		Reason: "locking",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is locked
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueAPI api.Issue
	DecodeJSON(t, resp, &issueAPI)
	assert.True(t, issueAPI.IsLocked)
}

func TestAPILockIssueReasonRequired(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	require.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Lock the Issue
	urlAPI := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "PUT", urlAPI, &api.IssueLockOption{}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPIDoubleLockIssueNotProcessable(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	require.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Lock the Issue
	urlAPI := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "PUT", urlAPI, &api.IssueLockOption{
		Reason: "locking",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is locked
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueAPI api.Issue
	DecodeJSON(t, resp, &issueAPI)
	assert.True(t, issueAPI.IsLocked)

	// Attempt to lock again
	req = NewRequestWithJSON(t, "PUT", urlAPI, &api.IssueLockOption{
		Reason: "locking",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusConflict)
}

func TestAPIUnlockIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	require.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Lock the Issue
	urlAPI := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "PUT", urlAPI, &api.IssueLockOption{
		Reason: "locking",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is locked
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueAPI api.Issue
	DecodeJSON(t, resp, &issueAPI)
	assert.True(t, issueAPI.IsLocked)

	// Unlock the Issue
	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is no longer locked
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &issueAPI)
	assert.False(t, issueAPI.IsLocked)
}

func TestAPIDoubleUnlockIssueUnprocessable(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	require.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Issue starts unlocked, try to unlock it again
	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusConflict)
}

func TestAPILockAndUnlockRequireWritePermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	require.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue) // Read token

	// Lock the Issue
	urlAPI := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)
	req := NewRequestWithJSON(t, "PUT", urlAPI, &api.IssueLockOption{
		Reason: "locking",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	// Ensure the issue was not locked
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueAPI api.Issue
	DecodeJSON(t, resp, &issueAPI)
	assert.False(t, issueAPI.IsLocked)

	// Attempt to unlock the Issue
	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}
