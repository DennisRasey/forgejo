// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"
	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
)

func TestAdminViewUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Admin user", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user1")
		req := NewRequest(t, "GET", "/admin/users")
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", "/admin/users?status_filter[is_2fa_enabled]=1")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		// 6th column is the 2FA column.
		// One user that has TOTP and another user that has WebAuthn.
		assert.Equal(t, 2, htmlDoc.Find(".admin-setting-content table tbody tr td:nth-child(6) .octicon-check").Length())

		// account type 5 is for remote users (eg. users from the federation)
		req = NewRequest(t, "GET", "/admin/users?status_filter[account_type]=5")
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)

		// Only one user (id 43) is a remote user
		assert.Equal(t, 1, htmlDoc.Find("table tbody tr").Length())
	})

	t.Run("Normal user", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/admin/users")
		session.MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("Anonymous user", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/admin/users")
		MakeRequest(t, req, http.StatusSeeOther)
	})
}

func TestAdminViewUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/admin/users/1")
	session.MakeRequest(t, req, http.StatusOK)

	session = loginUser(t, "user2")
	req = NewRequest(t, "GET", "/admin/users/1")
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAdminEditUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	testSuccessfulEdit(t, user_model.User{ID: 2, Name: "newusername", LoginName: "otherlogin", Email: "new@e-mail.gitea"})
}

func TestAdminEditUserHideEmail(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	userID := int64(2) // user2 from fixtures

	// Test setting hide_email to false
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/admin/users/%d/edit", userID), map[string]string{
		"user_name":  "user2",
		"login_name": "user2",
		"login_type": "0-0",
		"email":      "user2@example.com",
		"hide_email": "false",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
	assert.False(t, user.KeepEmailPrivate)

	// Verify the form now loads with hide_email not checked
	req = NewRequest(t, "GET", fmt.Sprintf("/admin/users/%d/edit", userID))
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, `input[name="hide_email"]:not([checked])`, true)

	// Test setting hide_email to true
	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/admin/users/%d/edit", userID), map[string]string{
		"user_name":  "user2",
		"login_name": "user2",
		"login_type": "0-0",
		"email":      "user2@example.com",
		"hide_email": "true",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})
	assert.True(t, user.KeepEmailPrivate)

	// Verify the form loads with hide_email checked
	req = NewRequest(t, "GET", fmt.Sprintf("/admin/users/%d/edit", userID))
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, `input[name="hide_email"][checked]`, true)
}

func TestAdminEditUserWebsite(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	user := forgery.CreateUser(t, nil)
	urlStr := fmt.Sprintf("/admin/users/%d/edit", user.ID)

	t.Run("an HTTPS website under default schemes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// changing website should work
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"user_name":  user.Name,
			"login_name": user.LoginName,
			"login_type": "0-0",
			"email":      user.Email,
			"website":    "https://codeberg.org",
		})
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assertHasFlashMessages(t, resp, "success")
	})

	t.Run("an H3 website under default schemes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// changing website should not work
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"user_name":  user.Name,
			"login_name": user.LoginName,
			"login_type": "0-0",
			"email":      user.Email,
			"website":    "h3://codeberg.org",
		})
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		flash := doc.Find("#flash-message").Text()
		assert.Equal(t, `Website"Url" is not a valid URL.`, strings.TrimSpace(flash))
	})

	t.Run("an H3 website under custom schemes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockProtect(&setting.Service.ValidSiteURLSchemes)()
		setting.Service.ValidSiteURLSchemes = append(setting.Service.ValidSiteURLSchemes, "h3")

		// changing website should work
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"user_name":  user.Name,
			"login_name": user.LoginName,
			"login_type": "0-0",
			"email":      user.Email,
			"website":    "h3://codeberg.org",
		})
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assertHasFlashMessages(t, resp, "success")
	})
}

func testSuccessfulEdit(t *testing.T, formData user_model.User) {
	makeRequest(t, formData, http.StatusSeeOther)
}

func makeRequest(t *testing.T, formData user_model.User, headerCode int) {
	session := loginUser(t, "user1")
	req := NewRequestWithValues(t, "POST", "/admin/users/"+strconv.Itoa(int(formData.ID))+"/edit", map[string]string{
		"user_name":  formData.Name,
		"login_name": formData.LoginName,
		"login_type": "0-0",
		"email":      formData.Email,
	})

	session.MakeRequest(t, req, headerCode)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: formData.ID})
	assert.Equal(t, formData.Name, user.Name)
	assert.Equal(t, formData.LoginName, user.LoginName)
	assert.Equal(t, formData.Email, user.Email)
}

func TestAdminDeleteUser(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestAdminDeleteUser")()
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	userID := int64(1000)

	unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{PosterID: userID})

	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/admin/users/%d/delete", userID), map[string]string{
		"purge": "true",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	assertUserDeleted(t, userID, true)
	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestSourceId(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	testUser23 := &user_model.User{
		Name:        "@ausersourceid23@example.net",
		LoginName:   "@ausersourceid23@example.net",
		Email:       "ausersourceid23@example.com",
		Passwd:      "ausersourceid23password",
		Type:        user_model.UserTypeRemoteUser,
		LoginType:   auth_model.Plain,
		LoginSource: 23,
	}
	defer createUser(t.Context(), t, testUser23)()

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin)

	// Historic background: The user was previously called "ausersourceid23", but the
	// test started failing on PostgreSQL specifically because of another federated user
	// in a fixture called @federated@example.net - this did not apply to other database
	// engines. Said user's username began with an 'a' so that it comes up on top, so, we
	// simply made another federated user that starts with '@a' here as an easy way out.
	req := NewRequest(t, "GET", "/api/v1/admin/users?limit=1").AddTokenAuth(token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var users []api.User
	DecodeJSON(t, resp, &users)
	assert.Len(t, users, 1)
	assert.Equal(t, "@ausersourceid23@example.net", users[0].UserName)

	// Now our new user should not be in the list, because we filter by source_id 0
	req = NewRequest(t, "GET", "/api/v1/admin/users?limit=1&source_id=0").AddTokenAuth(token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &users)
	assert.Len(t, users, 1)

	// SQLite sorts uppercase strings differently to MYSQL and PostgreSQL
	if setting.Database.Type.IsSQLite3() {
		assert.Equal(t, "User44", users[0].UserName)
	} else {
		assert.Equal(t, "imported", users[0].UserName)
	}

	// Now our new user should be in the list, because we filter by source_id 23
	req = NewRequest(t, "GET", "/api/v1/admin/users?limit=1&source_id=23").AddTokenAuth(token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &users)
	assert.Len(t, users, 1)
	assert.Equal(t, "@ausersourceid23@example.net", users[0].UserName)
}

func TestAdminViewUsersSorted(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	createTimestamp := time.Now().Unix() - 1000
	updateTimestamp := time.Now().Unix() - 500
	sess := db.GetEngine(t.Context())

	// Create 10 users with login source 44
	for i := int64(1); i <= 10; i++ {
		name := "sorttest" + strconv.Itoa(int(i))
		user := &user_model.User{
			Name:        name,
			LowerName:   name,
			LoginName:   name,
			Email:       name + "@example.com",
			Passwd:      name + ".password",
			Type:        user_model.UserTypeIndividual,
			LoginType:   auth_model.OAuth2,
			LoginSource: 44,
			CreatedUnix: timeutil.TimeStamp(createTimestamp - i),
			UpdatedUnix: timeutil.TimeStamp(updateTimestamp - i),
		}
		if _, err := sess.NoAutoTime().Insert(user); err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin)

	type testCase struct {
		loginSource   int64
		sortType      string
		expectedUsers []string
	}

	testCases := []testCase{
		{0, "newest", []string{"User44", "imported", "user40", "user39"}},
		{0, "oldest", []string{"user1", "user2", "user4", "user5"}},
		{44, "recentupdate", []string{"sorttest1", "sorttest2", "sorttest3", "sorttest4"}},
		{44, "leastupdate", []string{"sorttest10", "sorttest9", "sorttest8", "sorttest7"}},
	}

	// SQLite sorts uppercase strings differently to MYSQL and PostgreSQL
	if setting.Database.Type.IsSQLite3() {
		testCases = append(testCases, testCase{0, "alphabetically", []string{"User44", "imported", "the_34-user.with.all.allowedChars", "user1"}})
		testCases = append(testCases, testCase{0, "reversealphabetically", []string{"user9", "user8", "user5", "user40"}})
	} else {
		testCases = append(testCases, testCase{0, "alphabetically", []string{"imported", "the_34-user.with.all.allowedChars", "user1", "user10"}})
		testCases = append(testCases, testCase{0, "reversealphabetically", []string{"user9", "user8", "user5", "User44"}})
	}

	for _, testCase := range testCases {
		req := NewRequest(
			t,
			"GET",
			fmt.Sprintf("/api/v1/admin/users?sort=%s&limit=4&source_id=%d",
				testCase.sortType,
				testCase.loginSource),
		).AddTokenAuth(token)
		resp := session.MakeRequest(t, req, http.StatusOK)

		var users []api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 4)
		for i, user := range users {
			assert.Equalf(t, testCase.expectedUsers[i], user.UserName, "Sort type: %s, index %d", testCase.sortType, i)
		}
	}
}
