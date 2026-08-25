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
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgMembersPage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	testPage := "/org/org3/members"

	t.Run("Guest PoV", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		doc := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		/* No interactive buttons - though such evaluation is easy to break in rename */
		assert.Equal(t, 0, doc.Find(".members .list .link-action").Length())
		assert.Equal(t, 0, doc.Find(".members .list .delete-button").Length())
		/* Guests cannot add members to organizations */
		doc.AssertElement(t, "#add-org-member-button", false)
	})

	t.Run("Guest PoV with empty members list", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// hide the visibility of all public members of org3
		require.NoError(t, organization.ChangeOrgUserStatus(db.DefaultContext, 3, 2, false))
		require.NoError(t, organization.ChangeOrgUserStatus(db.DefaultContext, 3, 28, false))

		doc := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/org/org3/members"), http.StatusOK).Body)
		doc.AssertElement(t, ".empty-placeholder", true)
	})

	t.Run("Member PoV", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user4") // user4 is a member of org3
		doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		/* Interactive buttons are only available for own entry in the list */
		assert.Equal(t, 1, doc.Find(".members .list .link-action").Length())
		assert.Equal(t, 1, doc.Find(".members .list .delete-button").Length())
		/* Adding new members is not possible, as the member isn't an owner */
		doc.AssertElement(t, "#add-org-member-button", false)
	})

	t.Run("Owner PoV", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user2") // user2 owns org3
		doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)
		/* Interactive buttons are available for all entries in the list (> 2) */
		assert.Less(t, 2, doc.Find(".members .list .link-action").Length())
		assert.Less(t, 2, doc.Find(".members .list .delete-button").Length())
		/* Adding new members is possible */
		assert.Equal(t, "Add member", strings.TrimSpace(doc.Find("#add-org-member-button").Text()))
	})

	t.Run("Owner PoV with ADD_MEMBERS_BY_INVITATIONS setting on", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Service.AddMembersByInvitations, true)()

		session := loginUser(t, "user2") // user2 owns org3
		doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", testPage), http.StatusOK).Body)

		/* Inviting new members is possible */
		assert.Equal(t, "Invite member", strings.TrimSpace(doc.Find("#add-org-member-button").Text()))
	})
}

func TestOrgAddMember(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team1 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	team2 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team1.OrgID, team1.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	isMember, err = organization.IsTeamMember(db.DefaultContext, team2.OrgID, team2.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	session := loginUser(t, "user2")

	teamURL := fmt.Sprintf("/org/%s/members", org.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":    "2",
		"uname":  user.LoginName,
		"team_1": "on",
		"team_2": "on",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, teamURL, resp.Header().Get("Location"))

	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	isMember, err = organization.IsTeamMember(db.DefaultContext, team1.OrgID, team1.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
	isMember, err = organization.IsTeamMember(db.DefaultContext, team2.OrgID, team2.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestOrgAddMemberToNoTeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isOrgMember, err := org.IsOrgMember(db.DefaultContext, user.ID)
	require.NoError(t, err)
	assert.False(t, isOrgMember)

	session := loginUser(t, "user2")

	teamURL := fmt.Sprintf("/org/%s/members", org.Name)
	resp := session.MakeRequest(t, NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":   "2",
		"uname": user.LoginName,
	}), http.StatusSeeOther)
	assert.Equal(t, teamURL, resp.Header().Get("Location"))

	doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body)
	assert.Contains(t, strings.TrimSpace(doc.Find(".flash-error").Text()), "Organization members must belong to at least one team.")

	isOrgMember, err = org.IsOrgMember(db.DefaultContext, user.ID)
	require.NoError(t, err)
	assert.False(t, isOrgMember)
}

func TestOrgAddMemberToForeignTeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	foreignTeam := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 5})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isOrgMember, err := org.IsOrgMember(db.DefaultContext, user.ID)
	require.NoError(t, err)
	assert.False(t, isOrgMember)

	isForeignTeamMember, err := organization.IsTeamMember(db.DefaultContext, foreignTeam.OrgID, foreignTeam.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isForeignTeamMember)

	session := loginUser(t, "user2")

	teamURL := fmt.Sprintf("/org/%s/members", org.Name)
	// This test scenario is here for security purposes as the UI will not offer adding the user to
	// a foreign team in the first place (but a malicious request could be performed in other ways).
	resp := session.MakeRequest(t, NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":    "2",
		"uname":  user.LoginName,
		"team_5": "on",
	}), http.StatusSeeOther)
	assert.Equal(t, teamURL, resp.Header().Get("Location"))

	doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body)
	// This error message isn't specific to this "exploit" attempt, but shows that no action was taken.
	assert.Contains(t, strings.TrimSpace(doc.Find(".flash-error").Text()), "Organization members must belong to at least one team.")

	isOrgMember, err = org.IsOrgMember(db.DefaultContext, user.ID)
	require.NoError(t, err)
	assert.False(t, isOrgMember)

	isForeignTeamMember, err = organization.IsTeamMember(db.DefaultContext, foreignTeam.OrgID, foreignTeam.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isForeignTeamMember)
}

func TestOrgAddMemberWithoutProperRights(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isOrgMember, err := org.IsOrgMember(db.DefaultContext, user.ID)
	require.NoError(t, err)
	assert.False(t, isOrgMember)

	session := loginUser(t, user.Name) // user5 is not a member of this org, so it cannot add anyone to it

	teamURL := fmt.Sprintf("/org/%s/members", org.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":    "5",
		"uname":  user.LoginName,
		"team_2": "on",
	})
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestOrgAddExistingMemberFails(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})

	members, _, err := organization.FindOrgMembers(db.DefaultContext, &organization.FindOrgMembersOpts{
		OrgID: org.ID,
	})
	require.NoError(t, err)
	assert.Len(t, members, 2)
	isOrgMember, err := org.IsOrgMember(db.DefaultContext, user.ID)
	require.NoError(t, err)
	assert.True(t, isOrgMember)
	isTeamMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isTeamMember)

	session := loginUser(t, "user2")

	teamURL := fmt.Sprintf("/org/%s/members", org.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":    "2",
		"uname":  user.LoginName,
		"team_2": "on",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, teamURL, resp.Header().Get("Location"))
	doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", teamURL), http.StatusOK).Body)
	assert.Contains(t, strings.TrimSpace(doc.Find(".flash-error").Text()), "This user is already a member of the organization.")

	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	isTeamMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isTeamMember)
}

func TestOrgAddMemberGeneratesAnInvite(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.AddMembersByInvitations, true)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team1 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	team2 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team1.OrgID, team1.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	isMember, err = organization.IsTeamMember(db.DefaultContext, team2.OrgID, team2.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	session := loginUser(t, "user2")

	membersURL := fmt.Sprintf("/org/%s/members", org.Name)
	req := NewRequestWithValues(t, "POST", membersURL+"/action/add", map[string]string{
		"uid":    "2",
		"uname":  user.LoginName,
		"team_1": "on",
		"team_2": "on",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, membersURL, resp.Header().Get("Location"))

	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	isMember, err = organization.IsTeamMember(db.DefaultContext, team1.OrgID, team1.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	isMember, err = organization.IsTeamMember(db.DefaultContext, team2.OrgID, team2.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	unittest.AssertExistsAndLoadBean(t, &organization.TeamInvite{TeamID: team1.ID, InviterID: 2, InvitedID: optional.Some(user.ID)})
	unittest.AssertExistsAndLoadBean(t, &organization.TeamInvite{TeamID: team2.ID, InviterID: 2, InvitedID: optional.Some(user.ID)})

	doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", membersURL), http.StatusOK).Body)
	assert.Contains(t, strings.TrimSpace(doc.Find(".flash-info").Text()), "User successfully invited to the organization.")
}
