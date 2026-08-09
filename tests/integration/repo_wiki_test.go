// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestWikiSearchContent(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/wiki/search?q=This")
	resp := MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	res := doc.Find(".item > b").Map(func(_ int, el *goquery.Selection) string {
		return el.Text()
	})
	assert.Equal(t, []string{
		"Home",
		"Page With Spaced Name",
		"Page With Unescaped Special Chars: (1)",
		"Unescaped File",
	}, res)
}

func TestWikiBranchNormalize(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	username := "user2"
	session := loginUser(t, username)
	settingsURLStr := "/user2/repo1/settings"

	assertNormalizeButton := func(present bool) {
		req := NewRequest(t, "GET", settingsURLStr) //.AddTokenAuth(token)
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		htmlDoc.AssertElement(t, "button[data-modal='#rename-wiki-branch-modal']", present)
	}

	// By default the repo wiki branch is empty
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Empty(t, repo.WikiBranch)

	// This means we default to setting.Repository.DefaultBranch
	assert.Equal(t, setting.Repository.DefaultBranch, repo.GetWikiBranchName())

	// Which further means that the "Normalize wiki branch" parts do not appear on settings
	assertNormalizeButton(false)

	// Lets rename the branch!
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	repoURLStr := fmt.Sprintf("/api/v1/repos/%s/%s", username, repo.Name)
	wikiBranch := "wiki"
	req := NewRequestWithJSON(t, "PATCH", repoURLStr, &api.EditRepoOption{
		WikiBranch: &wikiBranch,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// The wiki branch should now be changed
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, wikiBranch, repo.GetWikiBranchName())

	// And as such, the button appears!
	assertNormalizeButton(true)

	// Invoking the normalization renames the wiki branch back to the default
	req = NewRequestWithValues(t, "POST", settingsURLStr, map[string]string{
		"action":    "rename-wiki-branch",
		"repo_name": repo.FullName(),
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Equal(t, setting.Repository.DefaultBranch, repo.GetWikiBranchName())
	assertNormalizeButton(false)
}

func TestWikiTOC(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	username := "user2"
	session := loginUser(t, username)

	t.Run("Link in heading", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithValues(t, "POST", "/user2/repo1/wiki/Home?action=_edit", map[string]string{
			"title":   "Home",
			"content": "# [Helpdesk](Helpdesk)",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		req = NewRequest(t, "GET", "/user2/repo1/wiki/Home")
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		assert.Equal(t, "Helpdesk", htmlDoc.Find(".wiki-content-toc a").Text())
	})
}

func canEditWiki(t *testing.T, username, url string, canEdit bool) {
	t.Helper()
	// t.Parallel()

	req := NewRequest(t, "GET", url)

	var resp *httptest.ResponseRecorder
	if username != "" {
		session := loginUser(t, username)
		resp = session.MakeRequest(t, req, http.StatusOK)
	} else {
		resp = MakeRequest(t, req, http.StatusOK)
	}
	doc := NewHTMLParser(t, resp.Body)
	res := doc.Find(`a[href^="` + url + `"]`).Map(func(_ int, el *goquery.Selection) string {
		return el.AttrOr("href", "")
	})
	found := false
	for _, href := range res {
		if strings.HasSuffix(href, "?action=_new") {
			if !canEdit {
				t.Errorf("unexpected edit link: %s", href)
			}
			found = true
			break
		}
	}
	if canEdit {
		assert.True(t, found, "could not find ?action=_new link among %v", res)
	}
}

func TestWikiPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("default settings", func(t *testing.T) {
		t.Run("anonymous", func(t *testing.T) {
			canEditWiki(t, "", "/user5/repo4/wiki", false)
		})
		t.Run("owner", func(t *testing.T) {
			canEditWiki(t, "user5", "/user5/repo4/wiki", true)
		})
		t.Run("collaborator", func(t *testing.T) {
			canEditWiki(t, "user4", "/user5/repo4/wiki", true)
			canEditWiki(t, "user29", "/user5/repo4/wiki", true)
		})
		t.Run("other user", func(t *testing.T) {
			canEditWiki(t, "user2", "/user5/repo4/wiki", false)
		})
	})

	t.Run("saved unchanged settings", func(t *testing.T) {
		session := loginUser(t, "user5")
		req := NewRequestWithValues(t, "POST", "/user5/repo4/settings/units", map[string]string{
			"enable_wiki": "on",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		t.Run("anonymous", func(t *testing.T) {
			canEditWiki(t, "", "/user5/repo4/wiki", false)
		})
		t.Run("owner", func(t *testing.T) {
			canEditWiki(t, "user5", "/user5/repo4/wiki", true)
		})
		t.Run("collaborator", func(t *testing.T) {
			canEditWiki(t, "user4", "/user5/repo4/wiki", true)
			canEditWiki(t, "user29", "/user5/repo4/wiki", true)
		})
		t.Run("other user", func(t *testing.T) {
			canEditWiki(t, "user2", "/user5/repo4/wiki", false)
		})
	})

	t.Run("globally writable", func(t *testing.T) {
		session := loginUser(t, "user5")
		req := NewRequestWithValues(t, "POST", "/user5/repo4/settings/units", map[string]string{
			"enable_wiki":             "on",
			"globally_writeable_wiki": "on",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		t.Run("anonymous", func(t *testing.T) {
			canEditWiki(t, "", "/user5/repo4/wiki", false)
		})
		t.Run("owner", func(t *testing.T) {
			canEditWiki(t, "user5", "/user5/repo4/wiki", true)
		})
		t.Run("collaborator", func(t *testing.T) {
			canEditWiki(t, "user4", "/user5/repo4/wiki", true)
			canEditWiki(t, "user29", "/user5/repo4/wiki", true)
		})
		t.Run("other user", func(t *testing.T) {
			canEditWiki(t, "user2", "/user5/repo4/wiki", true)
		})
	})
}
