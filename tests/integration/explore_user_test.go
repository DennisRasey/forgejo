// Copyright 2024 The Gitea Authors. All rights reserved.
// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

func TestExploreUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Set the default sort order
	defer test.MockVariableValue(&setting.UI.ExploreDefaultSort, "reversealphabetically")()

	cases := []struct{ sortOrder, expected string }{
		{"", "?sort=" + setting.UI.ExploreDefaultSort + "&q="},
		{"newest", "?sort=newest&q="},
		{"oldest", "?sort=oldest&q="},
		{"alphabetically", "?sort=alphabetically&q="},
		{"reversealphabetically", "?sort=reversealphabetically&q="},
	}
	for _, c := range cases {
		req := NewRequest(t, "GET", "/explore/users?sort="+c.sortOrder)
		resp := MakeRequest(t, req, http.StatusOK)
		h := NewHTMLParser(t, resp.Body)
		href, _ := h.Find(`.list-header details.dropdown > .content > ul > li > a.active[href^="?sort="]`).Attr("href")
		assert.Equal(t, c.expected, href)
	}

	// these sort orders shouldn't be supported, to avoid leaking user activity
	cases404 := []string{
		"/explore/users?sort=lastlogin",
		"/explore/users?sort=reverselastlogin",
		"/explore/users?sort=leastupdate",
		"/explore/users?sort=reverseleastupdate",
	}
	for _, c := range cases404 {
		req := NewRequest(t, "GET", c).SetHeader("Accept", "text/html")
		MakeRequest(t, req, http.StatusNotFound)
	}

	t.Run("REQUIRE_SIGNIN_VIEW", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Service.RequireSignInView, true)()
		req := NewRequest(t, "GET", "/explore/users")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/user/login", resp.Header().Get("Location"))
	})

	t.Run("[explore].REQUIRE_SIGNIN_VIEW", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Service.Explore.RequireSigninView, true)()
		req := NewRequest(t, "GET", "/explore/users")
		resp := MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/user/login", resp.Header().Get("Location"))
	})
}
