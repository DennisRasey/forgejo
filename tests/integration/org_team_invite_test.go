// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"
	"forgejo.org/services/mailer"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgTeamEmailInvite(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()
	mailerCalled := false
	defer test.MockVariableValue(&mailer.SendAsync, func(msgs ...*mailer.Message) {
		assert.Len(t, msgs, 1)
		assert.Equal(t, "user5@example.com", msgs[0].To)
		assert.Equal(t, "User One has invited you to join the <<<< >> >> > >> > >>> >> organization", msgs[0].Subject)
		assert.Contains(t, msgs[0].Body, "This invitation will expire in 14 days")
		mailerCalled = true
	})()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":   "1",
		"uname": user.Email,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// check that an invite email was sent
	assert.True(t, mailerCalled)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	require.NoError(t, err)
	assert.Len(t, invites, 1)

	session = loginUser(t, user.Name)

	// get the invite page
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", inviteURL)
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	// check the button exists
	submitButton := doc.Find(`button:contains('Join')`).Length()
	assert.Equal(t, 1, submitButton)

	// join the team
	req = NewRequest(t, "POST", inviteURL)
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
	publicMembership, err := organization.IsPublicMembership(db.DefaultContext, team.OrgID, user.ID)
	require.NoError(t, err)
	// we didn't check the hide_membership checkbox, so the membership is public
	assert.True(t, publicMembership)
}

func TestOrgTeamEmailInviteWithHiddenMembership(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	inviter := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	// create the invite
	invite, err := organization.CreateTeamInviteForUser(db.DefaultContext, inviter, user, team)
	require.NoError(t, err)

	session := loginUser(t, user.Name)

	// get the invite page
	inviteURL := fmt.Sprintf("/org/invite/%s", invite.Token)
	req := NewRequest(t, "GET", inviteURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	// check the button exists
	submitButton := doc.Find(`button:contains('Join')`).Length()
	assert.Equal(t, 1, submitButton)
	// check that the hide_membership checkbox exists
	hideMembershipCheckbox := doc.Find(`#hide_membership`).Length()
	assert.Equal(t, 1, hideMembershipCheckbox)

	// join the team
	req = NewRequestWithValues(t, "POST", inviteURL, map[string]string{"hide_membership": "on"})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
	publicMembership, err := organization.IsPublicMembership(db.DefaultContext, team.OrgID, user.ID)
	require.NoError(t, err)
	// we checked the hide_membership checkbox, so the membership is private
	assert.False(t, publicMembership)
}

// Check that users are redirected to accept the invitation correctly after login
func TestOrgTeamEmailInviteRedirectsExistingUser(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":   "1",
		"uname": user.Email,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	require.NoError(t, err)
	assert.Len(t, invites, 1)

	// accept the invite
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/login?redirect_to=%s", url.QueryEscape(inviteURL)))
	resp = MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithValues(t, "POST", "/user/login", map[string]string{
		"user_name": "user5",
		"password":  "password",
	})
	for _, c := range resp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	// complete the login process
	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session = emptyTestSession(t)
	baseURL, err := url.Parse(setting.AppURL)
	require.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	// make the request
	req = NewRequest(t, "POST", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

// Check that newly signed up users are redirected to accept the invitation correctly
func TestOrgTeamEmailInviteRedirectsNewUser(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":   "1",
		"uname": "doesnotexist@example.com",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	require.NoError(t, err)
	assert.Len(t, invites, 1)

	// accept the invite
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/sign_up?redirect_to=%s", url.QueryEscape(inviteURL)))
	resp = MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"user_name": "doesnotexist",
		"email":     "doesnotexist@example.com",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	for _, c := range resp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	// complete the signup process
	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session = emptyTestSession(t)
	baseURL, err := url.Parse(setting.AppURL)
	require.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	// make the redirected request
	req = NewRequest(t, "POST", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the new user
	newUser, err := user_model.GetUserByName(db.DefaultContext, "doesnotexist")
	require.NoError(t, err)

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, newUser.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

// Check that users are redirected correctly after confirming their email
func TestOrgTeamEmailInviteRedirectsNewUserWithActivation(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}
	defer test.MockVariableValue(&setting.Service.RegisterEmailConfirm, true)()
	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":   "1",
		"uname": "doesnotexist@example.com",
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	require.NoError(t, err)
	assert.Len(t, invites, 1)

	// accept the invite
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/sign_up?redirect_to=%s", url.QueryEscape(inviteURL)))
	inviteResp := MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"user_name": "doesnotexist",
		"email":     "doesnotexist@example.com",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	for _, c := range inviteResp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = MakeRequest(t, req, http.StatusOK)

	user, err := user_model.GetUserByName(db.DefaultContext, "doesnotexist")
	require.NoError(t, err)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Header()["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	session = emptyTestSession(t)
	baseURL, err := url.Parse(setting.AppURL)
	require.NoError(t, err)
	session.jar.SetCookies(baseURL, cr.Cookies())

	code, err := user.GenerateEmailAuthorizationCode(db.DefaultContext, auth.UserActivation)
	require.NoError(t, err)

	req = NewRequestWithValues(t, "POST", "/user/activate?code="+url.QueryEscape(code), map[string]string{
		"password": "examplePassword!1",
	})

	// use the cookies set by the signup request
	for _, c := range inviteResp.Result().Cookies() {
		req.AddCookie(c)
	}

	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	// should be redirected to accept the invite
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	req = NewRequest(t, "POST", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

// Test that a logged-in user who navigates to the sign-up link is then redirected using redirect_to
// For example: an invite may have been created before the user account was created, but they may be
// accepting the invite after having created an account separately
func TestOrgTeamEmailInviteRedirectsExistingUserWithLogin(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	// create the invite
	session := loginUser(t, "user1")

	teamURL := fmt.Sprintf("/org/%s/teams/%s", org.Name, team.Name)
	req := NewRequestWithValues(t, "POST", teamURL+"/action/add", map[string]string{
		"uid":   "1",
		"uname": user.Email,
	})
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	// get the invite token
	invites, err := organization.GetInvitesByTeamID(db.DefaultContext, team.ID)
	require.NoError(t, err)
	assert.Len(t, invites, 1)

	// note: the invited user has logged in
	session = loginUser(t, "user5")

	// accept the invite (note: this uses the sign_up url)
	inviteURL := fmt.Sprintf("/org/invite/%s", invites[0].Token)
	req = NewRequest(t, "GET", fmt.Sprintf("/user/sign_up?redirect_to=%s", url.QueryEscape(inviteURL)))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, inviteURL, test.RedirectURL(resp))

	// make the request
	req = NewRequest(t, "POST", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	req = NewRequest(t, "GET", test.RedirectURL(resp))
	session.MakeRequest(t, req, http.StatusOK)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
}

// Test that a user can accept a team invite linked to their existing account
func TestOrgTeamEmailInviteExistingUser(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	team1 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	team2 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	inviter := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team1.OrgID, team1.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	isMember, err = organization.IsTeamMember(db.DefaultContext, team2.OrgID, team2.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	// create the invites
	invite1, err := organization.CreateTeamInviteForUser(db.DefaultContext, inviter, user, team1)
	require.NoError(t, err)
	invite2, err := organization.CreateTeamInviteForUser(db.DefaultContext, inviter, user, team2)
	require.NoError(t, err)

	// log in the invited user
	session := loginUser(t, "user5")

	// view the first invite
	inviteURL1 := fmt.Sprintf("/org/invite/%s", invite1.Token)
	req := NewRequest(t, "GET", inviteURL1)
	session.MakeRequest(t, req, http.StatusOK)

	// accept the invite
	req = NewRequest(t, "POST", inviteURL1)
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	// after the first invite is accepted, the user is directly redirected to the next invite in the same org
	inviteURL2 := fmt.Sprintf("/org/invite/%s", invite2.Token)
	assert.Equal(t, test.RedirectURL(resp), inviteURL2)
	req = NewRequest(t, "GET", inviteURL2)
	session.MakeRequest(t, req, http.StatusOK)

	// the user has become a member of the first team
	isMember, err = organization.IsTeamMember(db.DefaultContext, team1.OrgID, team1.ID, user.ID)
	require.NoError(t, err)
	assert.True(t, isMember)
	// the second invite wasn't accepted yet, so the user isn't a member
	isMember, err = organization.IsTeamMember(db.DefaultContext, team2.OrgID, team2.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
}

// Test that a user cannot accept an invite if it was meant for another user
func TestOrgTeamEmailInviteCannotBeAcceptedByOtherUser(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	inviter := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	invited := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	attacker := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 11})

	isMember, err := organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, invited.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, attacker.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	// create the invite for the invited user
	invite, err := organization.CreateTeamInviteForUser(db.DefaultContext, inviter, invited, team)
	require.NoError(t, err)

	// log in as the attacker
	session := loginUser(t, attacker.Name)

	// viewing the invite doesn't work
	inviteURL := fmt.Sprintf("/org/invite/%s", invite.Token)
	req := NewRequest(t, "GET", inviteURL)
	session.MakeRequest(t, req, http.StatusNotFound)

	// accepting the invite doesn't either
	req = NewRequest(t, "POST", inviteURL)
	session.MakeRequest(t, req, http.StatusNotFound)

	// neither the invited user nor the attacker are part of the team
	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, invited.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, attacker.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
}

// Test that a user cannot accept an invite if it is expired
func TestOrgTeamEmailInviteExpired(t *testing.T) {
	if setting.MailService == nil {
		t.Skip()
		return
	}

	defer tests.PrepareTestEnv(t)()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	inviter := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	isMember, err := organization.IsTeamMember(t.Context(), team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)

	// create the invite
	invite, err := organization.CreateTeamInviteForUser(t.Context(), inviter, user, team)
	require.NoError(t, err)

	// set a deadline in the past, so that the invite is expired
	invite.ExpiryUnix = optional.Some(timeutil.TimeStamp(int64(timeutil.TimeStampNow()) - 500))
	_, err = db.GetEngine(t.Context()).Table("team_invite").Cols("expiry_unix").Update(
		&organization.TeamInvite{ExpiryUnix: optional.Some(timeutil.TimeStamp(int64(timeutil.TimeStampNow()) - 500))},
	)
	require.NoError(t, err)

	// log in the invited user
	session := loginUser(t, "user5")

	// view the invite
	inviteURL := fmt.Sprintf("/org/invite/%s", invite.Token)
	req := NewRequest(t, "GET", inviteURL)
	session.MakeRequest(t, req, http.StatusNotFound)

	// attempt to accept the invite despite the 404
	req = NewRequest(t, "POST", inviteURL)
	session.MakeRequest(t, req, http.StatusNotFound)

	isMember, err = organization.IsTeamMember(db.DefaultContext, team.OrgID, team.ID, user.ID)
	require.NoError(t, err)
	assert.False(t, isMember)
}
