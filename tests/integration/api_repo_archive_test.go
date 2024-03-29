// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIDownloadArchive(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user2.LowerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/archive/master.zip", user2.Name, repo.Name))
	resp := MakeRequest(t, NewRequest(t, "GET", link.String()).AddTokenAuth(token), http.StatusOK)
	bs, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Len(t, bs, 320)
	assert.EqualValues(t, "application/zip", resp.Header().Get("Content-Type"))

	link, _ = url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/archive/master.tar.gz", user2.Name, repo.Name))
	resp = MakeRequest(t, NewRequest(t, "GET", link.String()).AddTokenAuth(token), http.StatusOK)
	bs, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Len(t, bs, 266)
	assert.EqualValues(t, "application/gzip", resp.Header().Get("Content-Type"))

	link, _ = url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/archive/master.bundle", user2.Name, repo.Name))
	resp = MakeRequest(t, NewRequest(t, "GET", link.String()).AddTokenAuth(token), http.StatusOK)
	bs, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Len(t, bs, 382)
	assert.EqualValues(t, "application/octet-stream", resp.Header().Get("Content-Type"))

	link, _ = url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/archive/master", user2.Name, repo.Name))
	MakeRequest(t, NewRequest(t, "GET", link.String()).AddTokenAuth(token), http.StatusBadRequest)
}
