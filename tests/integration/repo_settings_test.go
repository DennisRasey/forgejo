// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"forgejo.org/models/db"
	git_model "forgejo.org/models/git"
	repo_model "forgejo.org/models/repo"
	unit_model "forgejo.org/models/unit"
	unit_tests "forgejo.org/models/unit/tests"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	app_context "forgejo.org/services/context"
	repo_service "forgejo.org/services/repository"
	user_service "forgejo.org/services/user"
	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoSettingsUnits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := forgery.CreateRepository(t, nil, nil)
	session := loginUser(t, repo.Owner.Name)

	req := NewRequest(t, "GET", fmt.Sprintf("%s/settings/units", repo.Link()))
	session.MakeRequest(t, req, http.StatusOK)
}

func TestRepoSettingsUpdateWebsite(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := forgery.CreateRepository(t, nil, nil)
	session := loginUser(t, repo.Owner.Name)
	urlStr := fmt.Sprintf("%s/settings", repo.Link())

	t.Run("an HTTPS website under default schemes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// changing website should work
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"action":    "update",
			"repo_name": repo.Name,
			"website":   "https://codeberg.org",
		})
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assertHasFlashMessages(t, resp, "success")
	})

	t.Run("an H3 website under default schemes", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// changing website should not work
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"action":    "update",
			"repo_name": repo.Name,
			"website":   "h3://codeberg.org",
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
			"action":    "update",
			"repo_name": repo.Name,
			"website":   "h3://codeberg.org",
		})
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assertHasFlashMessages(t, resp, "success")
	})
}

func TestRepoSettingsAdminOptions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := forgery.CreateUser(t, nil)
	repo := forgery.CreateRepository(t, user, nil)
	link := repo.Link()

	admin := forgery.CreateUser(t, &forgery.CreateUserOptions{
		IsAdmin: true,
	})

	hasAdminOpts := func(t *testing.T, doer string, admin bool) {
		session := loginUser(t, doer)

		req := NewRequest(t, "GET", fmt.Sprintf("%s/settings", link))
		resp := session.MakeRequest(t, req, http.StatusOK)
		html := NewHTMLParser(t, resp.Body)

		elems := html.doc.Find("button[name=request_reindex_type]")
		if !admin {
			assert.Empty(t, elems.Nodes)
			return
		}

		values := []string{"code", "issues", "stats"}
		if !setting.Indexer.RepoIndexerEnabled {
			values = values[1:]
		}
		elems.Each(func(i int, s *goquery.Selection) {
			attr, exists := s.Attr("value")
			require.True(t, exists)
			assert.Equal(t, values[i], attr)
		})
	}

	t.Run("guest", func(t *testing.T) {
		hasAdminOpts(t, user.Name, false)
	})

	t.Run("admin", func(t *testing.T) {
		hasAdminOpts(t, admin.Name, true)
	})
}

func TestRepoAddMoreUnitsHighlighting(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := forgery.CreateUser(t, nil)
	session := loginUser(t, user.Name)

	// Make sure there are no disabled repos in the settings!
	setting.Repository.DisabledRepoUnits = []string{}
	unit_model.LoadUnitConfig()

	// Create a known-good repo, with some units disabled.
	repo := forgery.CreateRepository(t, user, nil)
	forgery.EnableRepoUnits(t, repo,
		unit_model.TypeCode,
		unit_model.TypePullRequests,
		unit_model.TypeProjects,
		unit_model.TypeActions,
		unit_model.TypeIssues,
		unit_model.TypeWiki,
	)
	forgery.DisableRepoUnits(t, repo, unit_model.TypePackages)

	setUserHints := func(t *testing.T, hints bool) func() {
		saved := user.EnableRepoUnitHints

		require.NoError(t, user_service.UpdateUser(db.DefaultContext, user, &user_service.UpdateOptions{
			EnableRepoUnitHints: optional.Some(hints),
		}))

		return func() {
			require.NoError(t, user_service.UpdateUser(db.DefaultContext, user, &user_service.UpdateOptions{
				EnableRepoUnitHints: optional.Some(saved),
			}))
		}
	}

	assertHighlight := func(t *testing.T, page, uri string, highlighted bool) {
		t.Helper()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/settings%s", repo.Link(), page))
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		htmlDoc.AssertElement(t, fmt.Sprintf(".overflow-menu-items a[href='%s'].active", fmt.Sprintf("%s/settings%s", repo.Link(), uri)), highlighted)
	}

	t.Run("hints enabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer setUserHints(t, true)()

		t.Run("settings", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Visiting the /settings page, "Settings" is highlighted
			assertHighlight(t, "", "", true)
			// ...but "Add more" isn't.
			assertHighlight(t, "", "/units", false)
		})

		t.Run("units", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Visiting the /settings/units page, "Add more" is highlighted
			assertHighlight(t, "/units", "/units", true)
			// ...but "Settings" isn't.
			assertHighlight(t, "/units", "", false)
		})
	})

	t.Run("hints disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer setUserHints(t, false)()

		t.Run("settings", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Visiting the /settings page, "Settings" is highlighted
			assertHighlight(t, "", "", true)
			// ...but "Add more" isn't (it doesn't exist).
			assertHighlight(t, "", "/units", false)
		})

		t.Run("units", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Visiting the /settings/units page, "Settings" is highlighted
			assertHighlight(t, "/units", "", true)
			// ...but "Add more" isn't (it doesn't exist)
			assertHighlight(t, "/units", "/units", false)
		})
	})
}

func TestRepoAddMoreUnits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := forgery.CreateUser(t, nil)
	session := loginUser(t, user.Name)

	// Make sure there are no disabled repos in the settings!
	setting.Repository.DisabledRepoUnits = []string{}
	unit_model.LoadUnitConfig()

	// Create a known-good repo, with all units enabled.
	repo := forgery.CreateRepository(t, user, nil)
	forgery.EnableRepoUnits(t, repo,
		unit_model.TypeCode,
		unit_model.TypePullRequests,
		unit_model.TypeProjects,
		unit_model.TypePackages,
		unit_model.TypeActions,
		unit_model.TypeIssues,
		unit_model.TypeWiki,
	)

	assertAddMore := func(t *testing.T, present bool) {
		t.Helper()

		req := NewRequest(t, "GET", repo.Link())
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		htmlDoc.AssertElement(t, fmt.Sprintf("a[href='%s/settings/units']", repo.Link()), present)
	}

	t.Run("no add more with all units enabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		assertAddMore(t, false)
	})

	t.Run("add more if units can be enabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer func() {
			repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, []repo_model.RepoUnit{{
				RepoID: repo.ID,
				Type:   unit_model.TypePackages,
			}}, nil)
		}()

		// Disable the Packages unit
		err := repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, nil, []unit_model.Type{unit_model.TypePackages})
		require.NoError(t, err)

		assertAddMore(t, true)
	})

	t.Run("no add more if unit is globally disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer unit_tests.SaveUnits()()
		defer func() {
			repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, []repo_model.RepoUnit{{
				RepoID: repo.ID,
				Type:   unit_model.TypePackages,
			}}, nil)
		}()

		// Disable the Packages unit globally
		setting.Repository.DisabledRepoUnits = []string{"repo.packages"}
		unit_model.LoadUnitConfig()

		// Disable the Packages unit
		err := repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, nil, []unit_model.Type{unit_model.TypePackages})
		require.NoError(t, err)

		// The "Add more" link appears no more
		assertAddMore(t, false)
	})

	t.Run("issues & ext tracker globally disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer unit_tests.SaveUnits()()
		defer func() {
			repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, []repo_model.RepoUnit{{
				RepoID: repo.ID,
				Type:   unit_model.TypeIssues,
			}}, nil)
		}()

		// Disable both Issues and ExternalTracker units globally
		setting.Repository.DisabledRepoUnits = []string{"repo.issues", "repo.ext_issues"}
		unit_model.LoadUnitConfig()

		// Disable the Issues unit
		err := repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, nil, []unit_model.Type{unit_model.TypeIssues})
		require.NoError(t, err)

		// The "Add more" link appears no more
		assertAddMore(t, false)
	})
}

func TestProtectedBranch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := forgery.CreateUser(t, nil)
	repo := forgery.CreateRepository(t, user, &forgery.CreateRepositoryOptions{
		Files: forgery.FilesInit{},
	})
	session := loginUser(t, user.Name)

	t.Run("Add", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		link := fmt.Sprintf("/%s/settings/branches/edit", repo.FullName())

		req := NewRequestWithValues(t, "POST", link, map[string]string{
			"rule_name":   "master",
			"enable_push": "true",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// Verify it was added.
		unittest.AssertExistsIf(t, true, &git_model.ProtectedBranch{RuleName: "master", RepoID: repo.ID})
	})

	t.Run("Add duplicate", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		link := fmt.Sprintf("/%s/settings/branches/edit", repo.FullName())

		req := NewRequestWithValues(t, "POST", link, map[string]string{
			"rule_name":       "master",
			"require_signed_": "true",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		flashCookie := session.GetCookie(app_context.CookieNameFlash)
		assert.NotNil(t, flashCookie)
		assert.Equal(t, "error%3DThere%2Bis%2Balready%2Ba%2Brule%2Bfor%2Bthis%2Bset%2Bof%2Bbranches", flashCookie.Value)

		// Verify it wasn't added.
		unittest.AssertCount(t, &git_model.ProtectedBranch{RuleName: "master", RepoID: repo.ID}, 1)
	})
}

func TestDashboardLists(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, giteaURL *url.URL) {
		user := forgery.CreateUser(t, nil)
		repo := forgery.CreateRepository(t, user, nil)
		forgery.CreateIssue(t, user, repo, "test issue", "")
		forgery.CreatePullRequest(t, user, repo, "test pull request", "")
		forgery.CreateMilestone(t, repo, "test milestone", "")
		session := loginUser(t, user.Name)

		t.Run("no issues when issue unit disabled", func(t *testing.T) {
			forgery.DisableRepoUnits(t, repo, unit_model.TypeIssues)
			defer forgery.EnableRepoUnits(t, repo, unit_model.TypeIssues)

			req := NewRequest(t, "GET", "/issues")
			resp := session.MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)
			element := htmlDoc.doc.Find("h3")
			assert.NotNil(t, element)
			assert.Equal(t, "No results", element.Text())
		})

		t.Run("no PRs when PR unit disabled", func(t *testing.T) {
			forgery.DisableRepoUnits(t, repo, unit_model.TypePullRequests)
			defer forgery.EnableRepoUnits(t, repo, unit_model.TypePullRequests)

			req := NewRequest(t, "GET", "/pulls")
			resp := session.MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)
			element := htmlDoc.doc.Find("h3")
			assert.NotNil(t, element)
			assert.Equal(t, "No results", element.Text())
		})

		t.Run("no milestones with disabled issue, PR units", func(t *testing.T) {
			forgery.DisableRepoUnits(t, repo, unit_model.TypeIssues, unit_model.TypePullRequests)
			defer forgery.EnableRepoUnits(t, repo, unit_model.TypeIssues, unit_model.TypePullRequests)

			req := NewRequest(t, "GET", "/milestones")
			resp := session.MakeRequest(t, req, http.StatusOK)

			htmlDoc := NewHTMLParser(t, resp.Body)
			element := htmlDoc.doc.Find(".milestones strong")
			assert.NotNil(t, element)
			assert.Equal(t, "0", element.Text())
		})
	})
}
