// Copyright 2026 The Forgejo Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisibility(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// not logged in user
	req := NewRequest(t, "GET", "/org/org3/teams/team12creators")
	MakeRequest(t, req, http.StatusSeeOther)

	// not org member
	session := loginUser(t, "user5")
	req = NewRequest(t, "GET", "/org/org3/teams/team12creators")
	session.MakeRequest(t, req, http.StatusNotFound)

	// org member, not part of the team
	session = loginUser(t, "user4")
	req = NewRequest(t, "GET", "/org/org3/teams/team12creators")
	session.MakeRequest(t, req, http.StatusNotFound)

	// org member, part of the team
	session = loginUser(t, "user28")
	req = NewRequest(t, "GET", "/org/org3/teams/team12creators")
	session.MakeRequest(t, req, http.StatusOK)

	// org owner
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/org/org3/teams/team12creators")
	session.MakeRequest(t, req, http.StatusOK)

	// site admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/org/org3/teams/team12creators")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestPaginatedMembers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// To make sure that pagination kicks in even though the test team has few members
	defer test.MockVariableValue(&setting.UI.MembersPagingNum, 2)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 17})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 9})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})

	assert.GreaterOrEqual(t, org.NumMembers, 3)
	isOrgMember, err := organization.IsOrganizationMember(db.DefaultContext, org.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isOrgMember)
	isTeamMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isTeamMember)
	assert.Equal(t, org.ID, team.OrgID)

	session := loginUser(t, user.Name)

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.LowerName)
	newVar := session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body
	doc := NewHTMLParser(t, newVar)
	assert.Contains(t, strings.TrimSpace(doc.Find("a.item.navigation:contains('Next')").AttrOr("href", "")), fmt.Sprintf("%s?page=2", teamURL))
}

func TestPaginatedRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// To make sure that pagination kicks in even though the test team has few repos
	defer test.MockVariableValue(&setting.UI.User.RepoPagingNum, 2)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.GreaterOrEqual(t, team.NumRepos, 3)
	isOrgMember, err := organization.IsOrganizationMember(db.DefaultContext, org.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isOrgMember)
	isTeamMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isTeamMember)
	assert.Equal(t, org.ID, team.OrgID)

	session := loginUser(t, user.Name)

	teamURL := fmt.Sprintf("/org/%s/teams/%s/repositories", org.Name, team.LowerName)
	body := session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body
	doc := NewHTMLParser(t, body)
	assert.Contains(t, strings.TrimSpace(doc.Find("a.item.navigation:contains('Next')").AttrOr("href", "")), fmt.Sprintf("%s?page=2", teamURL))
}

func TestDisplayInvites(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestDisplayInvites")()
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	session := loginUser(t, user.Name)

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.LowerName)
	body := session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body
	doc := NewHTMLParser(t, body)

	// the two invited users are shown
	assert.Equal(t, "/user31", doc.Find("a:contains('user31')").AttrOr("href", ""))
	assert.Equal(t, 1, doc.Find("div.flex-item-main:contains('external_user@example.com')").Length())
	// the expired invitations are also shown
	assert.Equal(t, "/user30", doc.Find("a:contains('user30')").AttrOr("href", ""))
	assert.Equal(t, 1, doc.Find("div.flex-item-main:contains('other_ext_user@example.com')").Length())
	// there are buttons to remove any of those invitations
	assert.Equal(t, 4, doc.Find(fmt.Sprintf("form[action='%s/action/remove_invite'] button:contains('Remove')", teamURL)).Length())
	// and buttons to renew the expired ones
	assert.Equal(t, 2, doc.Find(fmt.Sprintf("form[action='%s/action/add'] button:contains('Renew')", teamURL)).Length())
}

func TestAddMembersByInvitations(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.AddMembersByInvitations, true)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	session := loginUser(t, user.Name)

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.LowerName)
	body := session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body

	// the button to add a team member says "invite"
	doc := NewHTMLParser(t, body)
	doc.AssertElement(t, "button.primary:contains('Invite to team')", true)

	// invite user "user31" to the team
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("%s/action/add", teamURL), map[string]string{
		"uname": "user31",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, teamURL, resp.Header().Get("Location"))

	// the invited user is listed on the team page
	body = session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body
	doc = NewHTMLParser(t, body)
	assert.Equal(t, "/user31", doc.Find("a:contains('user31')").AttrOr("href", ""))
}
