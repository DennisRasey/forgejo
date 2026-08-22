// Copyright 2026 The Forgejo Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"testing"

	"forgejo.org/tests"
)

func TestOrgTeams(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// not logged in user
	req := NewRequest(t, "GET", "/org/org3/teams")
	MakeRequest(t, req, http.StatusSeeOther)

	// not org member
	session := loginUser(t, "user5")
	req = NewRequest(t, "GET", "/org/org3/teams")
	session.MakeRequest(t, req, http.StatusNotFound)

	// org member, not part of the Owners team
	session = loginUser(t, "user28")
	req = NewRequest(t, "GET", "/org/org3/teams")
	doc := NewHTMLParser(t, session.MakeRequest(t, req, http.StatusOK).Body)
	doc.AssertElement(t, "a[href^='/org/org3/teams/owners'].text.black", false)
	doc.AssertElement(t, "a[href^='/org/org3/teams/team12creators'].text.black", true)
	// despite not being able to go to the page for the Owners team, the user still sees it exists:
	doc.AssertElement(t, "strong:contains('Owners')", true)

	// org owner
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/org/org3/teams")
	doc = NewHTMLParser(t, session.MakeRequest(t, req, http.StatusOK).Body)
	doc.AssertElement(t, "a[href^='/org/org3/teams/owners'].text.black", true)
	doc.AssertElement(t, "a[href^='/org/org3/teams/team12creators'].text.black", true)

	// site admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/org/org3/teams")
	doc = NewHTMLParser(t, session.MakeRequest(t, req, http.StatusOK).Body)
	doc.AssertElement(t, "a[href^='/org/org3/teams/owners'].text.black", true)
	doc.AssertElement(t, "a[href^='/org/org3/teams/team12creators'].text.black", true)
}
