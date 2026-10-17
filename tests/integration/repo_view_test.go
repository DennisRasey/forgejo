// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/test"
	"forgejo.org/routers/web/repo"
	"forgejo.org/services/context"
	"forgejo.org/services/contexttest"
	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createRepoAndGetContext(t *testing.T, user *user_model.User, filenames ...string) *context.Context {
	t.Helper()

	files := make(forgery.MapFS, len(filenames))
	for _, e := range filenames {
		if _, ok := files[e]; ok {
			t.Errorf("duplicated filename %q", e)
		}
		files[e] = forgery.MapFile("some readme content")
	}
	repo := forgery.CreateRepository(t, user, &forgery.CreateRepositoryOptions{
		Files: files,
	})
	ctx, _ := contexttest.MockContext(t, repo.FullName())
	ctx.SetParams(":id", fmt.Sprint(repo.ID))
	contexttest.LoadRepo(t, ctx, repo.ID)
	contexttest.LoadGitRepo(t, ctx)
	contexttest.LoadRepoCommit(t, ctx)
	t.Cleanup(func() {
		ctx.Repo.GitRepo.Close()
	})

	return ctx
}

func TestRepoView_FindReadme(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user := forgery.CreateUser(t, nil)
		t.Run("PrioOneLocalizedMdReadme", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user, "README.en.md", "README.en.org", "README.org", "README.txt", "README.tex", "README.md")

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Equal(t, "README.en.md", file.Name())
		})
		t.Run("PrioTwoMdReadme", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user, "README.en.org", "README.org", "README.txt", "README.tex", "README.md")

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Equal(t, "README.md", file.Name())
		})
		t.Run("PrioThreeLocalizedOrgReadme", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user, "README.en.org", "README.org", "README.txt", "README.tex")

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Equal(t, "README.en.org", file.Name())
		})
		t.Run("PrioFourOrgReadme", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user, "README.org", "README.txt", "README.tex")

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Equal(t, "README.org", file.Name())
		})
		t.Run("PrioFiveTxtReadme", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user, "README.txt", "README", "README.tex")

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Equal(t, "README.txt", file.Name())
		})
		t.Run("PrioSixWithoutExtensionReadme", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user, "README", "README.tex")

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Equal(t, "README", file.Name())
		})
		t.Run("PrioSevenAnyReadme", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user, "README.tex")

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Equal(t, "README.tex", file.Name())
		})
		t.Run("DoNotPickReadmeIfNonPresent", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			ctx := createRepoAndGetContext(t, user)

			tree, _ := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
			entries, _ := tree.ListEntries()
			_, file, _ := repo.FindReadmeFileInEntries(ctx, entries, false)

			assert.Nil(t, file)
		})
	})
}

func TestRepoViewFileLines(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, _ *url.URL) {
		repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{
				"test-1":          forgery.MapFile("No newline"),
				"test-2":          forgery.MapFile("No newline\n"),
				"test-3":          forgery.MapFile("Two\nlines"),
				"test-4":          forgery.MapFile("Really two\nlines\n"),
				"empty":           forgery.MapFile(""),
				"seemingly-empty": forgery.MapFile("\n"),
				"CITATION.cff":    forgery.MapFile(""),
			},
		})

		testEOL := func(t *testing.T, filename string, hasEOL bool) {
			t.Helper()
			req := NewRequestf(t, "GET", "%s/src/branch/main/%s", repo.Link(), filename)
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			fileInfo := htmlDoc.Find(".file-info").Text()
			if hasEOL {
				assert.NotContains(t, fileInfo, "No EOL")
			} else {
				assert.Contains(t, fileInfo, "No EOL")
			}
		}

		t.Run("No EOL", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			testEOL(t, "test-1", false)
			testEOL(t, "test-3", false)
		})

		t.Run("With EOL", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			testEOL(t, "test-2", true)
			testEOL(t, "test-4", true)
			testEOL(t, "empty", true)
			testEOL(t, "seemingly-empty", true)
		})
		t.Run("list", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequest(t, "GET", repo.Link())
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			nodes := htmlDoc.Find("#repo-files-table tr")
			t.Run("CITATION.cff", func(t *testing.T) {
				c, ok := nodes.Find(`.name a[title="CITATION.cff"] svg`).Attr("class")
				assert.True(t, ok, "could not find CITATION.cff line")
				assert.Contains(t, c, "octicon-cross-reference")
			})
		})
	})
}

func TestRepoWithTagButNoBranch(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		const tagName = "tag-test"

		user := forgery.CreateUser(t, nil)
		repo := forgery.CreateRepository(t, user, nil)

		// clone the repo
		repoPath := t.TempDir()
		doGitInitTestRepository(repoPath, git.Sha1ObjectFormat)(t)

		// create a commit
		generateCommitWithNewData(t, littleSize, repoPath, user.Email, user.Name, "newFile")

		// create tag
		_, _, err := git.NewCommand(git.DefaultContext, "tag").AddDynamicArguments(tagName).RunStdString(&git.RunOpts{Dir: repoPath})
		require.NoError(t, err)

		// push the tag
		u.Path = repo.FullName() + ".git"
		u.User = url.UserPassword(user.LowerName, userPassword)
		_, _, err = git.NewCommand(git.DefaultContext, "push", "--tags").AddDynamicArguments(u.String()).RunStdString(&git.RunOpts{Dir: repoPath})
		require.NoError(t, err)

		session := loginUser(t, user.Name)

		t.Run("repo view should redirect to the tag", func(t *testing.T) {
			req := NewRequest(t, "GET", "/"+repo.FullName())
			resp := session.MakeRequest(t, req, http.StatusSeeOther)
			assert.Equal(t, "/"+repo.FullName()+"/src/tag/"+tagName, test.RedirectURL(resp))
		})

		t.Run("/branches/list is an empty list, not nil", func(t *testing.T) {
			req := NewRequest(t, "GET", "/"+repo.FullName()+"/branches/list")
			resp := session.MakeRequest(t, req, http.StatusOK)

			respJSON := struct {
				Results []string
			}{}
			DecodeJSON(t, resp, &respJSON)
			assert.Equal(t, []string{}, respJSON.Results)
		})
	})
}
