// Copyright 2025 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	org_model "forgejo.org/models/organization"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	files_service "forgejo.org/services/repository/files"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgProfile(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		checkReadme := func(t *testing.T, title, readmeFilename string, expectedCount int) {
			t.Run(title, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				// Prepare the test repository
				org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

				var ops []*files_service.ChangeRepoFile
				op := "create"
				if readmeFilename != "README.md" {
					ops = append(ops, &files_service.ChangeRepoFile{
						Operation: "delete",
						TreePath:  "README.md",
					})
				} else {
					op = "update"
				}
				if readmeFilename != "" {
					ops = append(ops, &files_service.ChangeRepoFile{
						Operation:     op,
						TreePath:      readmeFilename,
						ContentReader: strings.NewReader("# Hi!\n"),
					})
				}

				_, _, f := tests.CreateDeclarativeRepo(t, org3, ".profile", nil, nil, ops)
				defer f()

				// Perform the test
				req := NewRequest(t, "GET", "/org3")
				resp := MakeRequest(t, req, http.StatusOK)

				doc := NewHTMLParser(t, resp.Body)
				readmeCount := doc.Find("#readme_profile").Length()

				assert.Equal(t, expectedCount, readmeCount)
			})
		}

		checkReadme(t, "No readme", "", 0)
		checkReadme(t, "README.md", "README.md", 1)
		checkReadme(t, "readme.md", "readme.md", 1)
		checkReadme(t, "ReadMe.mD", "ReadMe.mD", 1)
		checkReadme(t, "readme.org", "README.org", 1)
		checkReadme(t, "README.en-us.md", "README.en-us.md", 1)
		checkReadme(t, "README.en.md", "README.en.md", 1)
		checkReadme(t, "README.txt", "README.txt", 1)
		checkReadme(t, "README", "README", 1)
		checkReadme(t, "README.mdown", "README.mdown", 1)
		checkReadme(t, "README.i18n.md", "README.i18n.md", 1)
		checkReadme(t, "readmee", "readmee", 0)
		checkReadme(t, "test.md", "test.md", 0)

		t.Run("readme-size", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Prepare the test repository
			org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

			_, _, f := tests.CreateDeclarativeRepo(t, org3, ".profile", nil, nil, []*files_service.ChangeRepoFile{
				{
					Operation: "update",
					TreePath:  "README.md",
					ContentReader: strings.NewReader(`## Lorem ipsum
dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
## Ut enim ad minim veniam
quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum`),
				},
			})
			defer f()

			t.Run("full", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				defer test.MockVariableValue(&setting.UI.MaxDisplayFileSize, 500)()

				req := NewRequest(t, "GET", "/org3")
				resp := MakeRequest(t, req, http.StatusOK)
				assert.Contains(t, resp.Body.String(), "Ut enim ad minim veniam")
				assert.Contains(t, resp.Body.String(), "mollit anim id est laborum")
			})

			t.Run("truncated", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				defer test.MockVariableValue(&setting.UI.MaxDisplayFileSize, 146)()

				req := NewRequest(t, "GET", "/org3")
				resp := MakeRequest(t, req, http.StatusOK)
				assert.Contains(t, resp.Body.String(), "Ut enim ad minim")
				assert.NotContains(t, resp.Body.String(), "veniam")
			})
		})

		t.Run("More actions - feeds only", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			defer test.MockVariableValue(&setting.Other.EnableFeed, true)()
			defer test.MockVariableValue(&setting.Moderation.Enabled, false)()

			// Both guests and logged in users should see the feed option
			doc := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)
			doc.AssertElement(t, ".org-header details.dropdown a[href='/org3.rss']", true)
			doc.AssertElement(t, ".org-header details.dropdown a[href^='/report_abuse']", false)

			doc = NewHTMLParser(t, loginUser(t, "user10").MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)
			doc.AssertElement(t, ".org-header details.dropdown a[href='/org3.rss']", true)
			doc.AssertElement(t, ".org-header details.dropdown a[href^='/report_abuse']", false)
		})

		t.Run("More actions - none", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			defer test.MockVariableValue(&setting.Other.EnableFeed, false)()
			defer test.MockVariableValue(&setting.Moderation.Enabled, false)()

			// The dropdown won't appear if no entries are available, for both guests and logged in users
			doc := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)
			doc.AssertElement(t, ".org-header details.dropdown", false)

			doc = NewHTMLParser(t, loginUser(t, "user10").MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)
			doc.AssertElement(t, ".org-header details.dropdown", false)
		})

		t.Run("More actions - moderation", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			defer test.MockVariableValue(&setting.Other.EnableFeed, false)()
			defer test.MockVariableValue(&setting.Moderation.Enabled, true)()

			// The report option shouldn't be available to a guest
			doc := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)
			doc.AssertElement(t, ".org-header details.dropdown", false)

			// But should be available to a logged in user
			doc = NewHTMLParser(t, loginUser(t, "user10").MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)
			doc.AssertElement(t, ".org-header details.dropdown a[href^='/report_abuse']", true)

			// But the org owner shouldn't see the report option
			doc = NewHTMLParser(t, loginUser(t, "user1").MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)
			doc.AssertElement(t, ".org-header details.dropdown", false)
		})

		t.Run("Invitiation notification", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
			user10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
			ownersTeam := unittest.AssertExistsAndLoadBean(t, &org_model.Team{ID: 1})

			// create an invitation for user10 to org3
			invite, err := org_model.CreateTeamInviteForUser(t.Context(), user2, user10, ownersTeam)
			require.NoError(t, err)

			session := loginUser(t, "user10")
			doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK).Body)

			// the invitation is advertised on the org page
			doc.AssertElement(t, fmt.Sprintf("a[href^='/org/invite/%s']", invite.Token), true)
		})
	})
}
