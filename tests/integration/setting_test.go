// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/container"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/translation"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingShowUserEmailExplore(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	showUserEmail := setting.UI.ShowUserEmail
	setting.UI.ShowUserEmail = true

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/explore/users?sort=alphabetically")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find(".explore.users").Text(),
		"user34@example.com",
	)

	setting.UI.ShowUserEmail = false

	req = NewRequest(t, "GET", "/explore/users?sort=alphabetically")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t,
		htmlDoc.doc.Find(".explore.users").Text(),
		"user34@example.com",
	)

	setting.UI.ShowUserEmail = showUserEmail
}

func TestSettingShowUserEmailProfile(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	showUserEmail := setting.UI.ShowUserEmail

	// user1: keep_email_private = false, user2: keep_email_private = true

	setting.UI.ShowUserEmail = true

	// user1 can see own visible email
	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/user1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	// user1 can not see user2's hidden email
	req = NewRequest(t, "GET", "/user2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	// Should only contain if the user visits their own profile page
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@example.com")

	// user2 can see user1's visible email
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.Contains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	// user2 cannot see own hidden email
	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user2")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user2@example.com")

	setting.UI.ShowUserEmail = false

	// user1 cannot see own (now hidden) email
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/user1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	assert.NotContains(t, htmlDoc.doc.Find(".user.profile").Text(), "user1@example.com")

	setting.UI.ShowUserEmail = showUserEmail
}

func TestSettingLandingPage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockProtect(&setting.LandingPageURL)()

	setting.LandingPageURL = setting.LandingPageHome
	req := NewRequest(t, "GET", "/")
	MakeRequest(t, req, http.StatusOK)

	setting.LandingPageURL = setting.LandingPageExplore
	req = NewRequest(t, "GET", "/")
	resp := MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/explore", resp.Header().Get("Location"))

	setting.LandingPageURL = setting.LandingPageOrganizations
	req = NewRequest(t, "GET", "/")
	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/explore/organizations", resp.Header().Get("Location"))

	setting.LandingPageURL = setting.LandingPageLogin
	req = NewRequest(t, "GET", "/")
	resp = MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/user/login", resp.Header().Get("Location"))
}

func TestSettingSecurityAuthSource(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	active := addAuthSource(t, authSourcePayloadGitLabCustom("gitlab-active"))
	activeExternalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    "12345",
		UserID:        user.ID,
		LoginSourceID: active.ID,
	}
	err := user_model.LinkExternalToUser(db.DefaultContext, user, activeExternalLoginUser)
	require.NoError(t, err)

	inactive := addAuthSource(t, authSourcePayloadGitLabCustom("gitlab-inactive"))
	inactiveExternalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    "5678",
		UserID:        user.ID,
		LoginSourceID: inactive.ID,
	}
	err = user_model.LinkExternalToUser(db.DefaultContext, user, inactiveExternalLoginUser)
	require.NoError(t, err)

	// mark the authSource as inactive
	inactive.IsActive = false
	err = auth_model.UpdateSource(db.DefaultContext, inactive)
	require.NoError(t, err)

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "user/settings/security")
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `gitlab-active`)
	assert.Contains(t, resp.Body.String(), `gitlab-inactive`)
}

func TestSettingSecurityTwoFactorRequirement(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	locale := translation.NewLocale("en-US")

	runTest := func(t *testing.T, user *user_model.User, forceTOTP, showReroll, showUnroll bool) {
		t.Helper()
		defer unittest.AssertSuccessfulDelete(t, &auth_model.TwoFactor{UID: user.ID})

		useTOTP := forceTOTP || user.MustHaveTwoFactor()
		session := loginUserMaybeTOTP(t, user, useTOTP)

		resp := session.MakeRequest(t, NewRequest(t, "GET", "user/settings/security"), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		htmlDoc.AssertSelection(t, htmlDoc.FindByText("a", locale.TrString("settings.twofa_reenroll")), showReroll)
		htmlDoc.AssertElement(t, "#disable-form", showUnroll)
		htmlDoc.AssertSelection(t, htmlDoc.FindByText("p", locale.TrString("settings.twofa_unroll_unavailable")), showReroll && !showUnroll)

		req := NewRequest(t, "POST", "user/settings/security/two_factor/disable")
		if user.MustHaveTwoFactor() {
			session.MakeRequest(t, req, http.StatusNotFound)
		} else {
			resp := session.MakeRequest(t, req, http.StatusSeeOther)
			assert.Equal(t, "/user/settings/security", resp.Header().Get("Location"))
		}
	}

	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	normalUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	restrictedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})

	t.Run("NoneTwoFactorRequirement", func(t *testing.T) {
		t.Run("no 2fa", func(t *testing.T) {
			runTest(t, adminUser, false, false, false)
			runTest(t, normalUser, false, false, false)
			runTest(t, restrictedUser, false, false, false)
		})

		t.Run("enabled 2fa", func(t *testing.T) {
			runTest(t, adminUser, true, true, true)
			runTest(t, normalUser, true, true, true)
			runTest(t, restrictedUser, true, true, true)
		})
	})

	t.Run("AllTwoFactorRequirement", func(t *testing.T) {
		defer test.MockVariableValue(&setting.GlobalTwoFactorRequirement, setting.AllTwoFactorRequirement)()

		runTest(t, adminUser, false, true, false)
		runTest(t, normalUser, false, true, false)
		runTest(t, restrictedUser, false, true, false)
	})

	t.Run("AdminTwoFactorRequirement", func(t *testing.T) {
		defer test.MockVariableValue(&setting.GlobalTwoFactorRequirement, setting.AdminTwoFactorRequirement)()

		t.Run("no 2fa", func(t *testing.T) {
			runTest(t, adminUser, false, true, false)
			runTest(t, normalUser, false, false, false)
			runTest(t, restrictedUser, false, false, false)
		})

		t.Run("enabled 2fa", func(t *testing.T) {
			runTest(t, adminUser, true, true, false)
			runTest(t, normalUser, true, true, true)
			runTest(t, restrictedUser, true, true, true)
		})
	})
}

func TestUserAvatarSizeNotice(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/user/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find("form div:has(input#new-avatar) .help").Text(),
		"Custom avatar may not exceed 1 MiB in size or be larger than 4096x4096 pixels")
}

func TestRepoAvatarSizeNotice(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find("form div:has(input[name=\"avatar\"]) .help").Text(),
		"Custom avatar may not exceed 1 MiB in size or be larger than 4096x4096 pixels")
}

func TestOrgAvatarSizeNotice(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/org/org3/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find("form div:has(input[name=\"avatar\"]) .help").Text(),
		"Custom avatar may not exceed 1 MiB in size or be larger than 4096x4096 pixels")
}

func TestAdminAvatarSizeNotice(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/admin/users/2/edit")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Contains(t,
		htmlDoc.doc.Find("form div:has(input[name=\"avatar\"]) .help").Text(),
		"Custom avatar may not exceed 1 MiB in size or be larger than 4096x4096 pixels")
}

func TestSSHKeyManagementStatus(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	locale := translation.NewLocale("en-US")

	t.Run("Enabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/settings/keys")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		htmlDoc.AssertElement(t, "#add-ssh-button", true)

		assert.NotContains(t,
			htmlDoc.doc.Find(".user-setting-content .ui.warning.message").Text(),
			locale.TrString("keys.ssh.disabled"),
		)
	})

	t.Run("Disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.Admin.UserDisabledFeatures, container.SetOf("manage_ssh_keys"))()

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/settings/keys")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		htmlDoc.AssertElement(t, "#add-ssh-button", false)

		assert.Contains(t,
			htmlDoc.doc.Find(".user-setting-content .ui.warning.message").Text(),
			locale.TrString("keys.ssh.disabled"),
		)
	})
}
