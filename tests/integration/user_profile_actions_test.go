// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/tests"
)

func TestUserProfileActions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admSel := `details.dropdown a[href^="/admin/users/"]`
	blockSel := `details.dropdown button[commandfor="block-user"]`
	reportSel := `details.dropdown a[href^="/report_abuse?type=user"]`

	t.Run("Guest user", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Moderation.Enabled, true)()

		// Can't do much
		page := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/user1"), http.StatusOK).Body)
		page.AssertElement(t, admSel, false)
		page.AssertElement(t, blockSel, false)
		page.AssertElement(t, reportSel, false)
	})

	t.Run("User blocking", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Moderation.Enabled, true)()

		session := loginUser(t, "user2")

		// Can block others
		page := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user1"), http.StatusOK).Body)
		page.AssertElement(t, blockSel, true)

		// Can't block self
		page = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body)
		page.AssertElement(t, blockSel, false)
	})

	// To decrease the amount of requests, admin and moderation assertions are squashed together

	t.Run("Moderation enabled, user is admin", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Moderation.Enabled, true)()

		session := loginUser(t, "user1")
		// The /admin/... link  advertized to admins on all profiles

		// Can report others
		page := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body)
		page.AssertElement(t, reportSel, true)
		page.AssertElement(t, admSel, true)

		// Can't report self
		page = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user1"), http.StatusOK).Body)
		page.AssertElement(t, reportSel, false)
		page.AssertElement(t, admSel, true)
	})

	t.Run("Moderation disabled, user isn't admin", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Moderation.Enabled, false)()

		session := loginUser(t, "user2")
		// The /admin/... link is not advertized to non-admins

		// Can report anyone
		page := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user1"), http.StatusOK).Body)
		page.AssertElement(t, reportSel, false)
		page.AssertElement(t, admSel, false)

		page = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user2"), http.StatusOK).Body)
		page.AssertElement(t, reportSel, false)
		page.AssertElement(t, admSel, false)
	})
}
