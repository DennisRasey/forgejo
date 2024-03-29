// Copyright 2021 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestCompareTag(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1...master")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".choose.branch .filter.dropdown")
	// A dropdown for both base and head.
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")

	req = NewRequest(t, "GET", "/user2/repo1/compare/invalid")
	resp = session.MakeRequest(t, req, http.StatusNotFound)
	assert.False(t, strings.Contains(resp.Body.String(), ">500<"), "expect 404 page not 500")
}

// Compare with inferred default branch (master)
func TestCompareDefault(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/compare/v1.1")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	selection := htmlDoc.doc.Find(".choose.branch .filter.dropdown")
	assert.Lenf(t, selection.Nodes, 2, "The template has changed")
}

// Ensure the comparison matches what we expect
func inspectCompare(t *testing.T, htmlDoc *HTMLDoc, diffCount int, diffChanges []string) {
	selection := htmlDoc.doc.Find("#diff-file-boxes").Children()

	assert.Lenf(t, selection.Nodes, diffCount, "Expected %v diffed files, found: %v", diffCount, len(selection.Nodes))

	for _, diffChange := range diffChanges {
		selection = htmlDoc.doc.Find(fmt.Sprintf("[data-new-filename=\"%s\"]", diffChange))
		assert.Lenf(t, selection.Nodes, 1, "Expected 1 match for [data-new-filename=\"%s\"], found: %v", diffChange, len(selection.Nodes))
	}
}

// Git commit graph for repo20
// * 8babce9 (origin/remove-files-b) Add a dummy file
// * b67e43a Delete test.csv and link_hi
// | * cfe3b3c (origin/remove-files-a) Delete test.csv and link_hi
// |/
// * c8e31bc (origin/add-csv) Add test csv file
// * 808038d (HEAD -> master, origin/master, origin/HEAD) Added test links

func TestCompareBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Inderect compare remove-files-b (head) with add-csv (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added
	req := NewRequest(t, "GET", "/user2/repo20/compare/add-csv...remove-files-b")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	diffCount := 3
	diffChanges := []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Inderect compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted, 'test.txt' is added

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-a...remove-files-b")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 3
	diffChanges = []string{"link_hi", "test.csv", "test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Inderect compare remove-files-a (head) with remove-files-b (base) branch
	//
	//	'link_hi' and 'test.csv' are deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b...remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 2
	diffChanges = []string{"link_hi", "test.csv"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)

	// Direct compare remove-files-b (head) with remove-files-a (base) branch
	//
	//	'test.txt' is deleted

	req = NewRequest(t, "GET", "/user2/repo20/compare/remove-files-b..remove-files-a")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)

	diffCount = 1
	diffChanges = []string{"test.txt"}

	inspectCompare(t, htmlDoc, diffCount, diffChanges)
}

func TestCompareWithPRsDisabled(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "recent-push", http.StatusSeeOther)
		testEditFile(t, session, "user1", "repo1", "recent-push", "README.md", "Hello recently!\n")

		repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, "user1", "repo1")
		assert.NoError(t, err)

		defer func() {
			// Reenable PRs on the repo
			err := repo_service.UpdateRepositoryUnits(db.DefaultContext, repo,
				[]repo_model.RepoUnit{{
					RepoID: repo.ID,
					Type:   unit_model.TypePullRequests,
				}},
				nil)
			assert.NoError(t, err)
		}()

		// Disable PRs on the repo
		err = repo_service.UpdateRepositoryUnits(db.DefaultContext, repo, nil,
			[]unit_model.Type{unit_model.TypePullRequests})
		assert.NoError(t, err)

		t.Run("branch view doesn't offer creating PRs", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/user1/repo1/branches")
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			htmlDoc.AssertElement(t, "a[href='/user1/repo1/compare/master...recent-push']", false)
		})

		t.Run("compare doesn't offer local branches", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/user2/repo1/compare/master...user1/repo1:recent-push")
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			branches := htmlDoc.Find(".choose.branch .menu .reference-list-menu.base-branch-list .item, .choose.branch .menu .reference-list-menu.base-tag-list .item")

			expectedPrefix := "user2:"
			for i := 0; i < len(branches.Nodes); i++ {
				assert.True(t, strings.HasPrefix(branches.Eq(i).Text(), expectedPrefix))
			}
		})

		t.Run("comparing against a disabled-PR repo is 404", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/user1/repo1/compare/master...recent-push")
			session.MakeRequest(t, req, http.StatusNotFound)
		})
	})
}
