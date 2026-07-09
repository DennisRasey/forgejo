// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/services/forms"
	issue_service "forgejo.org/services/issue"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIViewPulls(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls?state=all", repo.OwnerName, repo.Name).AddTokenAuth(ctx.Token)
	resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

	var pulls []*api.PullRequest
	DecodeJSON(t, resp, &pulls)
	if assert.Len(t, pulls, 3) {
		slices.SortFunc(pulls, func(a, b *api.PullRequest) int {
			return cmp.Compare(a.ID, b.ID)
		})

		assert.EqualValues(t, 1, pulls[0].ID)
		assert.EqualValues(t, 2, pulls[0].Index)

		assert.EqualValues(t, 2, pulls[1].ID)
		assert.EqualValues(t, 3, pulls[1].Index)

		assert.EqualValues(t, 5, pulls[2].ID)
		assert.EqualValues(t, 5, pulls[2].Index)
	}
}

func TestAPIPullsFiles(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	t.Run("Pull 1", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/2/files", repo.OwnerName, repo.Name).AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var changedFiles []*api.ChangedFile
		DecodeJSON(t, resp, &changedFiles)

		assert.Empty(t, changedFiles)
		assert.Equal(t, "0", resp.Header().Get("X-Total-Count"))
		assert.Equal(t, "false", resp.Header().Get("X-HasMore"))
	})

	t.Run("Pull 2", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/3/files", repo.OwnerName, repo.Name).AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var changedFiles []*api.ChangedFile
		DecodeJSON(t, resp, &changedFiles)

		if assert.Len(t, changedFiles, 2) {
			assert.Equal(t, "2", resp.Header().Get("X-Total-Count"))
			assert.Equal(t, "false", resp.Header().Get("X-HasMore"))

			assert.Equal(t, "3", changedFiles[0].Filename)
			assert.Empty(t, changedFiles[0].PreviousFilename)
			assert.Equal(t, "added", changedFiles[0].Status)
			assert.Equal(t, 1, changedFiles[0].Changes)
			assert.Equal(t, 1, changedFiles[0].Additions)
			assert.Equal(t, 0, changedFiles[0].Deletions)
			assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/contents/3?ref=5f22f7d0d95d614d25a5b68592adb345a4b5c7fd", changedFiles[0].ContentsURL)
			assert.Equal(t, setting.AppURL+"user2/repo1/raw/commit/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd/3", changedFiles[0].RawURL)
			assert.Equal(t, setting.AppURL+"user2/repo1/src/commit/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd/3", changedFiles[0].HTMLURL)

			assert.Equal(t, "iso-8859-1.txt", changedFiles[1].Filename)
			assert.Empty(t, changedFiles[1].PreviousFilename)
			assert.Equal(t, "added", changedFiles[1].Status)
			assert.Equal(t, 10, changedFiles[1].Changes)
			assert.Equal(t, 10, changedFiles[1].Additions)
			assert.Equal(t, 0, changedFiles[1].Deletions)
			assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/contents/iso-8859-1.txt?ref=5f22f7d0d95d614d25a5b68592adb345a4b5c7fd", changedFiles[1].ContentsURL)
			assert.Equal(t, setting.AppURL+"user2/repo1/raw/commit/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd/iso-8859-1.txt", changedFiles[1].RawURL)
			assert.Equal(t, setting.AppURL+"user2/repo1/src/commit/5f22f7d0d95d614d25a5b68592adb345a4b5c7fd/iso-8859-1.txt", changedFiles[1].HTMLURL)
		}
	})

	t.Run("Pull 5", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/5/files", repo.OwnerName, repo.Name).AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var changedFiles []*api.ChangedFile
		DecodeJSON(t, resp, &changedFiles)

		if assert.Len(t, changedFiles, 1) {
			assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))
			assert.Equal(t, "false", resp.Header().Get("X-HasMore"))

			assert.Equal(t, "File-WoW", changedFiles[0].Filename)
			assert.Empty(t, changedFiles[0].PreviousFilename)
			assert.Equal(t, "added", changedFiles[0].Status)
			assert.Equal(t, 1, changedFiles[0].Changes)
			assert.Equal(t, 1, changedFiles[0].Additions)
			assert.Equal(t, 0, changedFiles[0].Deletions)
			assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/contents/File-WoW?ref=62fb502a7172d4453f0322a2cc85bddffa57f07a", changedFiles[0].ContentsURL)
			assert.Equal(t, setting.AppURL+"user2/repo1/raw/commit/62fb502a7172d4453f0322a2cc85bddffa57f07a/File-WoW", changedFiles[0].RawURL)
			assert.Equal(t, setting.AppURL+"user2/repo1/src/commit/62fb502a7172d4453f0322a2cc85bddffa57f07a/File-WoW", changedFiles[0].HTMLURL)
		}
	})

	t.Run("Pull 7", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/repos/user2/commitsonpr/pulls/1/files?limit=3").AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var changedFiles []*api.ChangedFile
		DecodeJSON(t, resp, &changedFiles)

		if assert.Len(t, changedFiles, 3) {
			assert.Equal(t, "10", resp.Header().Get("X-Total-Count"))
			assert.Equal(t, "true", resp.Header().Get("X-HasMore"))
			assert.Equal(t, "1", resp.Header().Get("X-Page"))
			assert.Equal(t, "3", resp.Header().Get("X-PerPage"))
			assert.Equal(t, "4", resp.Header().Get("X-PageCount"))

			assert.Equal(t, "test1.txt", changedFiles[0].Filename)
			assert.Empty(t, changedFiles[0].PreviousFilename)
			assert.Equal(t, "added", changedFiles[0].Status)
			assert.Equal(t, 1, changedFiles[0].Changes)
			assert.Equal(t, 1, changedFiles[0].Additions)
			assert.Equal(t, 0, changedFiles[0].Deletions)
			assert.Equal(t, setting.AppURL+"api/v1/repos/user2/commitsonpr/contents/test1.txt?ref=9b93963cf6de4dc33f915bb67f192d099c301f43", changedFiles[0].ContentsURL)
			assert.Equal(t, setting.AppURL+"user2/commitsonpr/raw/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test1.txt", changedFiles[0].RawURL)
			assert.Equal(t, setting.AppURL+"user2/commitsonpr/src/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test1.txt", changedFiles[0].HTMLURL)

			assert.Equal(t, "test10.txt", changedFiles[1].Filename)
			assert.Empty(t, changedFiles[1].PreviousFilename)
			assert.Equal(t, "added", changedFiles[1].Status)
			assert.Equal(t, 1, changedFiles[1].Changes)
			assert.Equal(t, 1, changedFiles[1].Additions)
			assert.Equal(t, 0, changedFiles[1].Deletions)
			assert.Equal(t, setting.AppURL+"api/v1/repos/user2/commitsonpr/contents/test10.txt?ref=9b93963cf6de4dc33f915bb67f192d099c301f43", changedFiles[1].ContentsURL)
			assert.Equal(t, setting.AppURL+"user2/commitsonpr/raw/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test10.txt", changedFiles[1].RawURL)
			assert.Equal(t, setting.AppURL+"user2/commitsonpr/src/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test10.txt", changedFiles[1].HTMLURL)

			assert.Equal(t, "test2.txt", changedFiles[2].Filename)
			assert.Empty(t, changedFiles[2].PreviousFilename)
			assert.Equal(t, "added", changedFiles[2].Status)
			assert.Equal(t, 1, changedFiles[2].Changes)
			assert.Equal(t, 1, changedFiles[2].Additions)
			assert.Equal(t, 0, changedFiles[2].Deletions)
			assert.Equal(t, setting.AppURL+"api/v1/repos/user2/commitsonpr/contents/test2.txt?ref=9b93963cf6de4dc33f915bb67f192d099c301f43", changedFiles[2].ContentsURL)
			assert.Equal(t, setting.AppURL+"user2/commitsonpr/raw/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test2.txt", changedFiles[2].RawURL)
			assert.Equal(t, setting.AppURL+"user2/commitsonpr/src/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test2.txt", changedFiles[2].HTMLURL)
		}
	})
}

func TestAPIPullsFilesSkipTo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	req := NewRequest(t, "GET", "/api/v1/repos/user2/commitsonpr/pulls/1/files?skip-to=test10.txt&limit=2").AddTokenAuth(ctx.Token)
	resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

	var changedFiles []*api.ChangedFile
	DecodeJSON(t, resp, &changedFiles)

	if assert.Len(t, changedFiles, 2) {
		assert.Equal(t, "9", resp.Header().Get("X-Total-Count")) // skip-to removes the skipped files range from the total count
		assert.Equal(t, "true", resp.Header().Get("X-HasMore"))
		assert.Equal(t, "1", resp.Header().Get("X-Page"))
		assert.Equal(t, "2", resp.Header().Get("X-PerPage"))
		assert.Equal(t, "5", resp.Header().Get("X-PageCount"))

		assert.Equal(t, "test10.txt", changedFiles[0].Filename)
		assert.Empty(t, changedFiles[0].PreviousFilename)
		assert.Equal(t, "added", changedFiles[0].Status)
		assert.Equal(t, 1, changedFiles[0].Changes)
		assert.Equal(t, 1, changedFiles[0].Additions)
		assert.Equal(t, 0, changedFiles[0].Deletions)
		assert.Equal(t, setting.AppURL+"api/v1/repos/user2/commitsonpr/contents/test10.txt?ref=9b93963cf6de4dc33f915bb67f192d099c301f43", changedFiles[0].ContentsURL)
		assert.Equal(t, setting.AppURL+"user2/commitsonpr/raw/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test10.txt", changedFiles[0].RawURL)
		assert.Equal(t, setting.AppURL+"user2/commitsonpr/src/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test10.txt", changedFiles[0].HTMLURL)

		assert.Equal(t, "test2.txt", changedFiles[1].Filename)
		assert.Empty(t, changedFiles[1].PreviousFilename)
		assert.Equal(t, "added", changedFiles[1].Status)
		assert.Equal(t, 1, changedFiles[1].Changes)
		assert.Equal(t, 1, changedFiles[1].Additions)
		assert.Equal(t, 0, changedFiles[1].Deletions)
		assert.Equal(t, setting.AppURL+"api/v1/repos/user2/commitsonpr/contents/test2.txt?ref=9b93963cf6de4dc33f915bb67f192d099c301f43", changedFiles[1].ContentsURL)
		assert.Equal(t, setting.AppURL+"user2/commitsonpr/raw/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test2.txt", changedFiles[1].RawURL)
		assert.Equal(t, setting.AppURL+"user2/commitsonpr/src/commit/9b93963cf6de4dc33f915bb67f192d099c301f43/test2.txt", changedFiles[1].HTMLURL)
	}
}

func TestAPIViewPullsFilterByBaseHead(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	t.Run("FilterByBase", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls?state=all&base=master", repo.OwnerName, repo.Name).
			AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var pulls []*api.PullRequest
		DecodeJSON(t, resp, &pulls)
		assert.Len(t, pulls, 2)
		for _, pr := range pulls {
			assert.Equal(t, "master", pr.Base.Name)
		}
	})

	t.Run("FilterByHead", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls?state=all&head=branch2", repo.OwnerName, repo.Name).
			AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var pulls []*api.PullRequest
		DecodeJSON(t, resp, &pulls)
		assert.Len(t, pulls, 1)
		assert.Equal(t, "branch2", pulls[0].Head.Name)
	})

	t.Run("FilterByBaseAndHead", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls?state=all&base=master&head=branch2", repo.OwnerName, repo.Name).
			AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var pulls []*api.PullRequest
		DecodeJSON(t, resp, &pulls)
		assert.Len(t, pulls, 1)
		assert.Equal(t, "master", pulls[0].Base.Name)
		assert.Equal(t, "branch2", pulls[0].Head.Name)
	})

	t.Run("FilterByBaseNoMatch", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls?state=all&base=nonexistent", repo.OwnerName, repo.Name).
			AddTokenAuth(ctx.Token)
		resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

		var pulls []*api.PullRequest
		DecodeJSON(t, resp, &pulls)
		assert.Empty(t, pulls)
	})
}

func TestAPIViewPullsByBaseHead(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	ctx := NewAPITestContext(t, "user2", repo.Name, auth_model.AccessTokenScopeReadRepository)

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/master/branch2", owner.Name, repo.Name).
		AddTokenAuth(ctx.Token)
	resp := ctx.Session.MakeRequest(t, req, http.StatusOK)

	pull := &api.PullRequest{}
	DecodeJSON(t, resp, pull)
	assert.EqualValues(t, 3, pull.Index)
	assert.EqualValues(t, 2, pull.ID)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/master/branch-not-exist", owner.Name, repo.Name).
		AddTokenAuth(ctx.Token)
	ctx.Session.MakeRequest(t, req, http.StatusNotFound)
}

// TestAPIMergePullWIP ensures that we can't merge a WIP pull request
func TestAPIMergePullWIP(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{Status: issues_model.PullRequestStatusMergeable}, unittest.Cond("has_merged = ?", false))
	pr.LoadIssue(db.DefaultContext)
	issue_service.ChangeTitle(db.DefaultContext, pr.Issue, owner, setting.Repository.PullRequest.WorkInProgressPrefixes[0]+" "+pr.Issue.Title)

	// force reload
	pr.LoadAttributes(db.DefaultContext)

	assert.Contains(t, pr.Issue.Title, setting.Repository.PullRequest.WorkInProgressPrefixes[0])

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", owner.Name, repo.Name, pr.Index), &forms.MergePullRequestForm{
		MergeMessageField: pr.Issue.Title,
		Do:                string(repo_model.MergeStyleMerge),
	}).AddTokenAuth(token)

	MakeRequest(t, req, http.StatusMethodNotAllowed)
}

func TestAPICreatePullSuccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	// repo10 have code, pulls units.
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	// repo11 only have code unit but should still create pulls
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	owner11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo11.OwnerID})

	session := loginUser(t, owner11.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &api.CreatePullRequestOption{
		Head:  fmt.Sprintf("%s:master", owner11.Name),
		Base:  "master",
		Title: "create a failure pr",
	}).AddTokenAuth(token)
	res := MakeRequest(t, req, http.StatusCreated)
	MakeRequest(t, req, http.StatusUnprocessableEntity) // second request should fail

	pull := new(api.PullRequest)
	DecodeJSON(t, res, pull)

	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", pull.MergeBase)
	assert.Contains(t, pull.URL, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", owner10.Name, repo10.Name, pull.Index))
}

func TestAPICreatePullSameRepoSuccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner.Name, repo.Name), &api.CreatePullRequestOption{
		Head:  fmt.Sprintf("%s:pr-to-update", owner.Name),
		Base:  "master",
		Title: "successfully create a PR between branches of the same repository",
	}).AddTokenAuth(token)
	res := MakeRequest(t, req, http.StatusCreated)
	MakeRequest(t, req, http.StatusUnprocessableEntity) // second request should fail

	pull := new(api.PullRequest)
	DecodeJSON(t, res, pull)

	assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", pull.MergeBase)
}

func TestAPICreatePullWithFieldsSuccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// repo10 have code, pulls units.
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	// repo11 only have code unit but should still create pulls
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	owner11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo11.OwnerID})

	session := loginUser(t, owner11.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	opts := &api.CreatePullRequestOption{
		Head:      fmt.Sprintf("%s:master", owner11.Name),
		Base:      "master",
		Title:     "create a failure pr",
		Body:      "foobaaar",
		Milestone: 5,
		Assignees: []string{owner10.Name},
		Labels:    []int64{5},
	}

	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), opts).
		AddTokenAuth(token)

	res := MakeRequest(t, req, http.StatusCreated)
	pull := new(api.PullRequest)
	DecodeJSON(t, res, pull)

	assert.NotNil(t, pull.Milestone)
	assert.Equal(t, opts.Milestone, pull.Milestone.ID)
	if assert.Len(t, pull.Assignees, 1) {
		assert.Equal(t, opts.Assignees[0], owner10.Name)
	}
	assert.NotNil(t, pull.Labels)
	assert.Equal(t, opts.Labels[0], pull.Labels[0].ID)
}

func TestAPICreatePullWithFieldsFailure(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// repo10 have code, pulls units.
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})
	// repo11 only have code unit but should still create pulls
	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	owner11 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo11.OwnerID})

	session := loginUser(t, owner11.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	opts := &api.CreatePullRequestOption{
		Head: fmt.Sprintf("%s:master", owner11.Name),
		Base: "master",
	}

	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), opts).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Title = "is required"

	opts.Milestone = 666
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Milestone = 5

	opts.Assignees = []string{"qweruqweroiuyqweoiruywqer"}
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Assignees = []string{owner10.LoginName}

	opts.Labels = []int64{55555}
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	opts.Labels = []int64{5}
}

func TestAPIEditPull(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	owner10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo10.OwnerID})

	session := loginUser(t, owner10.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	title := "create a success pr"
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls", owner10.Name, repo10.Name), &api.CreatePullRequestOption{
		Head:  "develop",
		Base:  "master",
		Title: title,
	}).AddTokenAuth(token)
	apiPull := new(api.PullRequest)
	resp := MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, apiPull)
	assert.Equal(t, "master", apiPull.Base.Name)

	newTitle := "edit a this pr"
	newBody := "edited body"
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d", owner10.Name, repo10.Name, apiPull.Index)
	req = NewRequestWithJSON(t, http.MethodPatch, urlStr, &api.EditPullRequestOption{
		Base:  "feature/1",
		Title: newTitle,
		Body:  &newBody,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, apiPull)
	assert.Equal(t, "feature/1", apiPull.Base.Name)
	// check comment history
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: apiPull.ID})
	err := pull.LoadIssue(db.DefaultContext)
	require.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: pull.Issue.ID, OldTitle: title, NewTitle: newTitle})
	unittest.AssertExistsAndLoadBean(t, &issues_model.ContentHistory{IssueID: pull.Issue.ID, ContentText: newBody, IsFirstCreated: false})

	// verify the idempotency of a state change
	pullState := string(apiPull.State)
	req = NewRequestWithJSON(t, http.MethodPatch, urlStr, &api.EditPullRequestOption{
		State: &pullState,
	}).AddTokenAuth(token)
	apiPullIdempotent := new(api.PullRequest)
	resp = MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, apiPullIdempotent)
	assert.Equal(t, apiPull.State, apiPullIdempotent.State)

	req = NewRequestWithJSON(t, http.MethodPatch, urlStr, &api.EditPullRequestOption{
		Base: "not-exist",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIForkDifferentName(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Step 1: get a repo and a user that can fork this repo
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	// Step 2: fork this repo with another name
	forkName := "myfork"
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", owner.Name, repo.Name),
		&api.CreateForkOption{Name: &forkName}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusAccepted)

	// Step 3: make a PR onto the original repo, it should succeed
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/pulls?state=all", owner.Name, repo.Name),
		&api.CreatePullRequestOption{Head: user.Name + ":master", Base: "master", Title: "hi"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

func TestAPIPullDeleteBranchPerms(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, giteaURL *url.URL) {
		user2Session := loginUser(t, "user2")
		user4Session := loginUser(t, "user4")
		testRepoFork(t, user4Session, "user2", "repo1", "user4", "repo1")
		testEditFileToNewBranch(t, user2Session, "user2", "repo1", "master", "base-pr", "README.md", "Hello, World\n(Edited - base PR)\n")

		req := NewRequestWithValues(t, "POST", "/user4/repo1/compare/master...user2/repo1:base-pr", map[string]string{
			"title": "Testing PR",
		})
		resp := user4Session.MakeRequest(t, req, http.StatusOK)
		elem := strings.Split(test.RedirectURL(resp), "/")

		token := getTokenForLoggedInUser(t, user4Session, auth_model.AccessTokenScopeWriteRepository)
		req = NewRequestWithValues(t, "POST", "/api/v1/repos/user4/repo1/pulls/"+elem[4]+"/merge", map[string]string{
			"do":                        "merge",
			"delete_branch_after_merge": "on",
		}).AddTokenAuth(token)
		resp = user4Session.MakeRequest(t, req, http.StatusForbidden)

		type userResponse struct {
			Message string `json:"message"`
		}
		var bodyResp userResponse
		DecodeJSON(t, resp, &bodyResp)

		assert.Equal(t, "insufficient permission to delete head branch", bodyResp.Message)

		// Check that the branch still exist.
		req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/branches/base-pr").AddTokenAuth(token)
		user4Session.MakeRequest(t, req, http.StatusOK)
	})
}
