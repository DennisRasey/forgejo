// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/routers"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

func TestRepoStarUnstarUI(t *testing.T) {
	t.Helper()

	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user5")

	// Star the repo as user5
	req := NewRequest(t, "POST", "/user2/repo1/action/star")
	session.MakeRequest(t, req, http.StatusOK)

	// Load the repo home as user5
	req = NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Verify that the star button is now unstar
	htmlDoc := NewHTMLParser(t, resp.Body)
	actionButton := htmlDoc.Find("form[action='/user2/repo1/action/unstar']")
	assert.Equal(t, 1, actionButton.Length())
	text := strings.ToLower(actionButton.Find("button span.text").Text())
	assert.Equal(t, "unstar", text)

	listLink := htmlDoc.Find("a[href$='/stars']")
	ariaLabel, _ := listLink.Attr("aria-label")
	assert.Equal(t, "1 star", ariaLabel)

	// Load stargazers as user5
	req = NewRequestf(t, "GET", "/user2/repo1/stars")
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Verify that "user5" is among the stargazers
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, ".user-cards .list .card > a[href='/user5']", true)

	// Verify which user-cards elements are present
	htmlDoc.AssertElement(t, ".user-cards > .list", true)
	htmlDoc.AssertElement(t, ".user-cards > div", false)

	// Unstar the repo as user5
	req = NewRequest(t, "POST", "/user2/repo1/action/unstar")
	session.MakeRequest(t, req, http.StatusOK)

	// Load the repo home as user5
	req = NewRequest(t, "GET", "/user2/repo1")
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Verify that the star button is now back to star
	htmlDoc = NewHTMLParser(t, resp.Body)
	actionButton = htmlDoc.Find("form[action='/user2/repo1/action/star']")
	assert.Equal(t, 1, actionButton.Length())
	text = strings.ToLower(actionButton.Find("button span.text").Text())
	assert.Equal(t, "star", text)

	// Load stargazers as user5
	req = NewRequestf(t, "GET", "/user2/repo1/stars")
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Verify that "user5" is not among the stargazers
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, ".user-cards .list .item.ui.segment > a[href='/user2']", false)

	// Verify which user-cards elements are present
	htmlDoc.AssertElement(t, ".user-cards > .list", false)
	htmlDoc.AssertElement(t, ".user-cards > div", true)
}

func TestRepoWatchUnwatchUI(t *testing.T) {
	t.Helper()

	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user5")

	// Watch the repo as user5 (using watch/settings endpoint with all events)
	req := NewRequestWithValues(t, "POST", "/user2/repo1/action/watch/select", map[string]string{
		"watch_issues":        "true",
		"watch_pull_requests": "true",
		"watch_releases":      "true",
	})
	session.MakeRequest(t, req, http.StatusOK)

	// Load the repo home as user5
	req = NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Verify that the watch dropdown shows "Watching"
	htmlDoc := NewHTMLParser(t, resp.Body)
	watchButton := htmlDoc.Find("details.dropdown#watch-button summary span.text")
	assert.Equal(t, 1, watchButton.Length())
	text := strings.TrimSpace(watchButton.Text())
	assert.Equal(t, "Watching", text)

	listLink := htmlDoc.Find("a[href$='/watchers']")
	ariaLabel, _ := listLink.Attr("aria-label")
	assert.Equal(t, "5 watchers", ariaLabel)

	// Load watchers as user5
	req = NewRequestf(t, "GET", "/user2/repo1/watchers")
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Verify that "user5" is among the watchers
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, ".user-cards .list .card > a[href='/user5']", true)

	// Unwatch the repo as user5
	req = NewRequest(t, "POST", "/user2/repo1/action/unwatch")
	session.MakeRequest(t, req, http.StatusOK)

	// Load the repo home as user5
	req = NewRequest(t, "GET", "/user2/repo1")
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Verify that the watch dropdown shows "Watch"
	htmlDoc = NewHTMLParser(t, resp.Body)
	watchButton = htmlDoc.Find("details.dropdown#watch-button summary span.text")
	assert.Equal(t, 1, watchButton.Length())
	text = strings.TrimSpace(watchButton.Text())
	assert.Equal(t, "Watch", text)

	// Load watchers as user5
	req = NewRequestf(t, "GET", "/user2/repo1/watchers")
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Verify that "user5" is not among the watchers
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, ".user-cards .list .item.ui.segment > a[href='/user2']", false)
}

func TestDisabledStars(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Repository.DisableStars, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	t.Run("repo star, unstar", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "POST", "/user2/repo1/action/star")
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "POST", "/user2/repo1/action/unstar")
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("repo stargazers", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2/repo1/stars")
		MakeRequest(t, req, http.StatusNotFound)
	})
}
