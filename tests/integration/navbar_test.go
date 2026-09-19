// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"strings"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/translation"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

/* TestNavbarItems asserts go tmpl logic of navbar */
func TestNavbarItems(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// The navbar can be tested on any page, but preferably a lightweight one
	testPage := "/explore/organizations"
	locale := translation.NewLocale("en-US")

	adminUser := loginUser(t, "user1")
	regularUser := loginUser(t, "user2")

	t.Run(`"Create..." dropdown - migrations disallowed`, func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Repository.DisableMigrations, true)()

		page := NewHTMLParser(t, regularUser.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		page.AssertElement(t, `details.dropdown a[href="/repo/migrate"]`, false)
	})

	t.Run(`"Create..." dropdown - creating orgs disallowed`, func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Admin.DisableRegularOrgCreation, true)()

		// The restriction applies to a regular user
		page := NewHTMLParser(t, regularUser.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		page.AssertElement(t, `details.dropdown a[href="/org/create"]`, false)

		// The restriction does not apply to an admin
		page = NewHTMLParser(t, adminUser.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		page.AssertElement(t, `details.dropdown a[href="/org/create"]`, true)
	})

	t.Run(`"Create..." dropdown - default conditions`, func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Assert that items are present and their contents
		assertItems := func(t *testing.T, session *TestSession) {
			page := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
			links := page.Find(`#navbar .dropdown:has(summary[data-tooltip-content="Create…"]) .content`)
			assert.Equal(t, locale.TrString("new_repo.link"), strings.TrimSpace(links.Find(`a[href="/repo/create"]`).Text()))
			assert.Equal(t, locale.TrString("new_migrate.link"), strings.TrimSpace(links.Find(`a[href="/repo/migrate"]`).Text()))
			assert.Equal(t, locale.TrString("new_org.link"), strings.TrimSpace(links.Find(`a[href="/org/create"]`).Text()))
		}
		assertItems(t, regularUser)
		assertItems(t, adminUser)
	})

	t.Run(`User dropdown - stars are disabled`, func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Repository.DisableStars, true)()

		page := NewHTMLParser(t, regularUser.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		page.AssertElement(t, `details.dropdown a[href$="?tab=stars"]`, false)
	})

	t.Run(`User dropdown - instance in dev mode`, func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.IsProd, false)()

		page := NewHTMLParser(t, regularUser.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		page.AssertElement(t, `details.dropdown a[href="/-/demo"]`, true)

		testNavbarUserMenuActiveItem(t, regularUser, "/user/settings")
		testNavbarUserMenuActiveItem(t, adminUser, "/admin")
	})

	t.Run(`User dropdown - default conditions`, func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.AppDocsVer, func() string { return "v0.0" })()

		// What regular user sees
		assertions := []struct {
			selector string
			exists   bool
		}{
			{`details.dropdown a[href="/user2"]`, true},
			{`details.dropdown a[href="/user2?tab=stars"]`, true},
			{`details.dropdown a[href="/notifications/subscriptions"]`, true},
			{`details.dropdown a[href="/user/settings"]`, true},
			{`details.dropdown a[href="/admin"]`, false},
			{`details.dropdown a[href="/-/demo"]`, false},
			{`details.dropdown a[href="https://forgejo.org/docs/v0.0/"]`, true}, // derived from setting.AppDocsVer
			{`details.dropdown button[form="logout-user-action"]`, true},
			{`form#logout-user-action[action="/user/logout"]`, true},
		}
		page := NewHTMLParser(t, regularUser.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		for _, assertion := range assertions {
			page.AssertElement(t, assertion.selector, assertion.exists)
		}

		// What admin user sees
		assertions = []struct {
			selector string
			exists   bool
		}{
			{`details.dropdown a[href="/user1"]`, true},
			{`details.dropdown a[href="/user1?tab=stars"]`, true},
			{`details.dropdown a[href="/notifications/subscriptions"]`, true},
			{`details.dropdown a[href="/user/settings"]`, true},
			{`details.dropdown a[href="/admin"]`, true},
			{`details.dropdown a[href="/-/demo"]`, false},
			{`details.dropdown a[href="https://forgejo.org/docs/v0.0/"]`, true},
			{`details.dropdown button[form="logout-user-action"]`, true},
			{`form#logout-user-action[action="/user/logout"]`, true},
		}
		page = NewHTMLParser(t, adminUser.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		for _, assertion := range assertions {
			page.AssertElement(t, assertion.selector, assertion.exists)
		}

		testNavbarUserMenuActiveItem(t, regularUser, "/user/settings")
		testNavbarUserMenuActiveItem(t, adminUser, "/admin")
	})
}

// When visiting certain pages, the corresponding entry of user menu is highlighted
func testNavbarUserMenuActiveItem(t *testing.T, session *TestSession, url string) {
	page := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK).Body)
	// AssertElement will only pass if there's just one such element
	page.AssertElement(t, "#navbar details.dropdown li > .active", true)
}
