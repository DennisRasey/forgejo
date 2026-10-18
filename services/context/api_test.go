// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	perm_model "forgejo.org/models/perm"
	access_model "forgejo.org/models/perm/access"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/services/authz"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenAPILinks(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://localhost:3000/")()
	kases := map[string][]string{
		"api/v1/repos/jerrykan/example-repo/issues?state=all": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=2&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=1": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=2&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=2": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=3&state=all>; rel="next"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=5&state=all>; rel="last"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="first"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="prev"`,
		},
		"api/v1/repos/jerrykan/example-repo/issues?state=all&page=5": {
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=1&state=all>; rel="first"`,
			`<http://localhost:3000/api/v1/repos/jerrykan/example-repo/issues?page=4&state=all>; rel="prev"`,
		},
	}

	for req, response := range kases {
		u, err := url.Parse(setting.AppURL + req)
		require.NoError(t, err)

		p := u.Query().Get("page")
		curPage, _ := strconv.Atoi(p)

		links := genAPILinks(u, 100, 20, curPage)

		assert.Equal(t, links, response)
	}
}

func TestAcceptsGithubResponse(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		resp := httptest.NewRecorder()
		base, baseCleanUp := NewBaseContext(resp, req)
		t.Cleanup(baseCleanUp)
		ctx := &APIContext{Base: base}

		assert.False(t, ctx.AcceptsGithubResponse())
	})

	t.Run("Accepts Github", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Add("Accept", "application/vnd.github+json")
		resp := httptest.NewRecorder()
		base, baseCleanUp := NewBaseContext(resp, req)
		t.Cleanup(baseCleanUp)
		ctx := &APIContext{Base: base}

		assert.True(t, ctx.AcceptsGithubResponse())
	})

	t.Run("Accepts Github with params", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Add("Accept", "application/vnd.github.v3+json")
		resp := httptest.NewRecorder()
		base, baseCleanUp := NewBaseContext(resp, req)
		t.Cleanup(baseCleanUp)
		ctx := &APIContext{Base: base}

		assert.True(t, ctx.AcceptsGithubResponse())
	})
}

func TestIsUserSiteAdmin(t *testing.T) {
	makeCtx := func(t *testing.T, reducer authz.AuthorizationReducer) *APIContext {
		req := httptest.NewRequest("GET", "/", nil)
		resp := httptest.NewRecorder()
		base, baseCleanUp := NewBaseContext(resp, req)
		t.Cleanup(baseCleanUp)
		ctx := &APIContext{Base: base, reducer: reducer}
		// setup ctx with an admin, and the test cases will modify to false in various ways
		ctx.SetIsSigned(true)
		ctx.SetDoer(&user_model.User{IsAdmin: true})
		return ctx
	}

	defaultReducer := authz.NewMockAuthorizationReducer(t)
	defaultReducer.On("AllowAdminOverride").Return(true)

	t.Run("not authenticated", func(t *testing.T) {
		ctx := makeCtx(t, defaultReducer)
		ctx.SetIsSigned(false)
		assert.False(t, ctx.IsUserSiteAdmin())
	})

	t.Run("non-admin", func(t *testing.T) {
		ctx := makeCtx(t, defaultReducer)
		ctx.Doer().IsAdmin = false
		assert.False(t, ctx.IsUserSiteAdmin())
	})

	t.Run("admin", func(t *testing.T) {
		ctx := makeCtx(t, defaultReducer)
		assert.True(t, ctx.IsUserSiteAdmin())
	})

	t.Run("admin w/ reducer", func(t *testing.T) {
		reducer := authz.NewMockAuthorizationReducer(t)
		reducer.On("AllowAdminOverride").Return(false)
		ctx := makeCtx(t, reducer)
		assert.False(t, ctx.IsUserSiteAdmin())
	})
}

func TestIsUserRepoAdmin(t *testing.T) {
	makeCtx := func(t *testing.T, reducer authz.AuthorizationReducer) *APIContext {
		req := httptest.NewRequest("GET", "/", nil)
		resp := httptest.NewRecorder()
		base, baseCleanUp := NewBaseContext(resp, req)
		t.Cleanup(baseCleanUp)
		ctx := &APIContext{Base: base, reducer: reducer}
		// setup ctx with a repo admin, and the test cases will modify to false in various ways
		ctx.SetRepo(&Repository{Permission: access_model.Permission{AccessMode: perm_model.AccessModeAdmin}})
		return ctx
	}

	defaultReducer := authz.NewMockAuthorizationReducer(t)
	defaultReducer.On("AllowAdminOverride").Return(true)

	t.Run("non-admin", func(t *testing.T) {
		ctx := makeCtx(t, defaultReducer)
		ctx.Repo().Permission.AccessMode = perm_model.AccessModeWrite
		assert.False(t, ctx.IsUserRepoAdmin())
	})

	t.Run("admin", func(t *testing.T) {
		ctx := makeCtx(t, defaultReducer)
		assert.True(t, ctx.IsUserRepoAdmin())
	})

	t.Run("admin w/ reducer", func(t *testing.T) {
		reducer := authz.NewMockAuthorizationReducer(t)
		reducer.On("AllowAdminOverride").Return(false)
		ctx := makeCtx(t, reducer)
		assert.False(t, ctx.IsUserRepoAdmin())
	})
}
