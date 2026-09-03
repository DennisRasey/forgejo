// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"forgejo.org/models/unittest"
	base "forgejo.org/modules/migration"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/services/migrations/allowlist"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPagureDownloaderBlocksLocalhost(t *testing.T) {
	defer test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, false, func() { require.NoError(t, allowlist.Init()) })()

	u, _ := url.Parse("http://localhost")
	downloader := NewPagureDownloader(t.Context(), u, "", "test_repo")
	_, err := downloader.GetRepoInfo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can only call allowed HTTP servers")
}

func TestPagureDownloadRepoWithPublicIssues(t *testing.T) {
	defer test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, true, func() { require.NoError(t, allowlist.Init()) })()

	// Skip tests if Pagure token is not found
	cloneUser := os.Getenv("PAGURE_CLONE_USER")
	clonePassword := os.Getenv("PAGURE_CLONE_PASSWORD")
	apiUser := os.Getenv("PAGURE_API_USER")
	apiPassword := os.Getenv("PAGURE_API_TOKEN")

	fixtPath := "./testdata/pagure/full_download/unauthorized"
	server := unittest.NewMockWebServer(t, "https://pagure.io", fixtPath, false)
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	if clonePassword != "" || cloneUser != "" {
		serverURL.User = url.UserPassword(cloneUser, clonePassword)
	}
	serverURL.Path = "protop2g-test-srce.git"
	cloneAddr := serverURL.String()

	factory := &PagureDownloaderFactory{}
	downloader, err := factory.New(t.Context(), base.MigrateOptions{
		CloneAddr:    cloneAddr,
		AuthUsername: apiUser,
		AuthPassword: apiPassword,
	})
	require.NoError(t, err)

	repo, err := downloader.GetRepoInfo()
	require.NoError(t, err)

	// Testing repository contents migration
	assertRepositoryEqual(t, &base.Repository{
		Name:        "protop2g-test-srce",
		Owner:       "",
		Description: "The source namespace for the Pagure Exporter project to run tests against",
		CloneURL:    cloneAddr,
		OriginalURL: strings.ReplaceAll(cloneAddr, ".git", ""),
	}, repo)

	topics, err := downloader.GetTopics()
	require.NoError(t, err)

	// Testing repository topics migration
	assert.Equal(t, []string{"srce", "test", "gridhead", "protop2g"}, topics)

	// Testing labels migration
	labels, err := downloader.GetLabels()
	require.NoError(t, err)
	assert.Len(t, labels, 15)

	// Testing issue tickets probing
	issues, isEnd, err := downloader.GetIssues(1, 20)
	require.NoError(t, err)
	assert.True(t, isEnd)

	// Testing issue tickets migration
	assertIssuesEqual(t, []*base.Issue{
		{
			Number:     2,
			Title:      "This is the title of the second test issue",
			Content:    "This is the body of the second test issue",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "closed",
			Milestone:  "Milestone BBBB",
			Created:    time.Date(2023, time.October, 13, 4, 1, 16, 0, time.UTC),
			Updated:    time.Date(2025, time.June, 25, 6, 25, 57, 0, time.UTC),
			Closed:     new(time.Date(2025, time.June, 25, 6, 22, 59, 0, time.UTC)),
			Labels: []*base.Label{
				{
					Name: "cccc",
				},
				{
					Name: "dddd",
				},
				{
					Name: "Closed As/Complete",
				},
				{
					Name: "Priority/Rare",
				},
			},
		},
		{
			Number:     1,
			Title:      "This is the title of the first test issue",
			Content:    "This is the body of the first test issue",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "open",
			Milestone:  "Milestone AAAA",
			Created:    time.Date(2023, time.October, 13, 3, 57, 42, 0, time.UTC),
			Updated:    time.Date(2025, time.June, 25, 6, 25, 45, 0, time.UTC),
			Closed:     new(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)),
			Labels: []*base.Label{
				{
					Name: "aaaa",
				},
				{
					Name: "bbbb",
				},
				{
					Name: "Priority/Epic",
				},
			},
		},
	}, issues)

	// Testing comments under issue tickets
	comments, _, err := downloader.GetComments(issues[0])
	require.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2023, time.October, 13, 4, 3, 30, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue tagged with: cccc, dddd",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2023, time.October, 13, 4, 6, 4, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "The is the first comment under the second test issue",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2023, time.October, 13, 4, 6, 16, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "The is the second comment under the second test issue",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2023, time.October, 13, 4, 7, 12, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue status updated to: Closed (was: Open)",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.May, 8, 4, 50, 21, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue set to the milestone: Milestone BBBB",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 22, 52, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue set to the milestone: None (was: Milestone BBBB)\n- Issue status updated to: Open (was: Closed)",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 23, 0o2, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue close_status updated to: Complete\n- Issue status updated to: Closed (was: Open)",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 24, 34, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue set to the milestone: Milestone BBBB",
		},
		{
			IssueIndex: 2,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 25, 57, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue priority set to: Rare",
		},
	}, comments)

	prs, isEnd, err := downloader.GetPullRequests(1, 20)
	require.NoError(t, err)
	assert.True(t, isEnd)

	// Testing pull requests migrated
	assertPullRequestsEqual(t, []*base.PullRequest{
		{
			Number:     10,
			Title:      "Change the branch identity to `test-ffff` in the README.md file",
			Content:    "Signed-off-by: Akashdeep Dhar <akashdeep.dhar@gmail.com>",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "closed",
			Created:    time.Date(2025, time.May, 19, 6, 12, 45, 0, time.UTC),
			Updated:    time.Date(2025, time.May, 19, 6, 17, 11, 0, time.UTC),
			Closed:     new(time.Date(2025, time.May, 19, 6, 17, 11, 0, time.UTC)),
			MergedTime: new(time.Date(2025, time.May, 19, 6, 17, 11, 0, time.UTC)),
			Merged:     true,
			PatchURL:   server.URL + "/protop2g-test-srce/pull-request/10.patch",
			Labels: []*base.Label{
				{
					Name: "ffff",
				},
			},
			Head: base.PullRequestBranch{
				Ref:      "test-ffff",
				SHA:      "1a6ccc212aa958a0fe76155c2907c889969a7224",
				RepoName: "protop2g-test-srce",
				CloneURL: server.URL + "/protop2g-test-srce.git",
			},
			Base: base.PullRequestBranch{
				Ref:      "main",
				SHA:      "01b420e2964928a15f790f9b7c1a0053e7b5f0a5",
				RepoName: "protop2g-test-srce",
			},
		},
		{
			Number:     9,
			Title:      "Change the branch identity to `test-eeee` in the README.md file",
			Content:    "Signed-off-by: Akashdeep Dhar <akashdeep.dhar@gmail.com>",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "closed",
			Created:    time.Date(2025, time.May, 19, 6, 12, 41, 0, time.UTC),
			Updated:    time.Date(2025, time.May, 19, 6, 14, 3, 0, time.UTC),
			Closed:     new(time.Date(2025, time.May, 19, 6, 14, 3, 0, time.UTC)),
			MergedTime: new(time.Date(2025, time.May, 19, 6, 14, 3, 0, time.UTC)),
			Merged:     true,
			PatchURL:   server.URL + "/protop2g-test-srce/pull-request/9.patch",
			Labels: []*base.Label{
				{
					Name: "eeee",
				},
			},
			Head: base.PullRequestBranch{
				Ref:      "test-eeee",
				SHA:      "01b420e2964928a15f790f9b7c1a0053e7b5f0a5",
				RepoName: "protop2g-test-srce",
				CloneURL: server.URL + "/protop2g-test-srce.git",
			},
			Base: base.PullRequestBranch{
				Ref:      "main",
				SHA:      "3f12d300f62f1c5b8a1d3265bd85d61cf6d924d7",
				RepoName: "protop2g-test-srce",
			},
		},
		{
			Number:     8,
			Title:      "Change the branch identity to `test-dddd` in the README.md file",
			Content:    "Signed-off-by: Akashdeep Dhar <testaddr@testaddr.com>",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "closed",
			Created:    time.Date(2025, time.May, 5, 6, 45, 32, 0, time.UTC),
			Updated:    time.Date(2025, time.May, 5, 6, 54, 13, 0, time.UTC),
			Closed:     new(time.Date(2025, time.May, 5, 6, 54, 13, 0, time.UTC)),
			MergedTime: new(time.Date(2025, time.May, 5, 6, 54, 13, 0, time.UTC)), // THIS IS WRONG
			Merged:     false,
			PatchURL:   server.URL + "/protop2g-test-srce/pull-request/8.patch",
			Labels: []*base.Label{
				{
					Name: "dddd",
				},
			},
			Head: base.PullRequestBranch{
				Ref:      "test-dddd",
				SHA:      "0bc8b0c38e0790e9ef5c8d512a00b9c4dd048160",
				RepoName: "protop2g-test-srce",
				CloneURL: server.URL + "/protop2g-test-srce.git",
			},
			Base: base.PullRequestBranch{
				Ref:      "main",
				SHA:      "3f12d300f62f1c5b8a1d3265bd85d61cf6d924d7",
				RepoName: "protop2g-test-srce",
			},
		},
		{
			Number:     7,
			Title:      "Change the branch identity to `test-cccc` in the README.md file",
			Content:    "Signed-off-by: Akashdeep Dhar <testaddr@testaddr.com>",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "closed",
			Created:    time.Date(2025, time.May, 5, 6, 45, 6, 0, time.UTC),
			Updated:    time.Date(2025, time.May, 5, 6, 54, 3, 0, time.UTC),      // IT SHOULD BE NIL
			Closed:     new(time.Date(2025, time.May, 5, 6, 54, 3, 0, time.UTC)), // IT is CLOSED, Not MERGED so SHOULD NOT BE NIL
			MergedTime: new(time.Date(2025, time.May, 5, 6, 54, 3, 0, time.UTC)), // THIS IS WRONG
			Merged:     false,
			PatchURL:   server.URL + "/protop2g-test-srce/pull-request/7.patch",
			Labels: []*base.Label{
				{
					Name: "cccc",
				},
			},
			Head: base.PullRequestBranch{
				Ref:      "test-cccc",
				SHA:      "f1246e331cade9341b9e4f311b7a134f99893d21",
				RepoName: "protop2g-test-srce",
				CloneURL: server.URL + "/protop2g-test-srce.git",
			},
			Base: base.PullRequestBranch{
				Ref:      "main",
				SHA:      "3f12d300f62f1c5b8a1d3265bd85d61cf6d924d7",
				RepoName: "protop2g-test-srce",
			},
		},
		{
			Number:     6,
			Title:      "Change the branch identity to `test-bbbb` in the README.md file",
			Content:    "Signed-off-by: Akashdeep Dhar <testaddr@testaddr.com>",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "open",
			Created:    time.Date(2025, time.May, 5, 6, 44, 30, 0, time.UTC),
			Updated:    time.Date(2025, time.May, 19, 8, 30, 50, 0, time.UTC),
			Closed:     new(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)),
			MergedTime: new(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)),
			Merged:     false,
			PatchURL:   server.URL + "/protop2g-test-srce/pull-request/6.patch",
			Labels: []*base.Label{
				{
					Name: "bbbb",
				},
			},
			Head: base.PullRequestBranch{
				Ref:      "test-bbbb",
				SHA:      "2d40761dc53e6fa060ac49d88e1452c6751d4b1c",
				RepoName: "protop2g-test-srce",
				CloneURL: server.URL + "/protop2g-test-srce.git",
			},
			Base: base.PullRequestBranch{
				Ref:      "main",
				SHA:      "3f12d300f62f1c5b8a1d3265bd85d61cf6d924d7",
				RepoName: "protop2g-test-srce",
			},
		},
		{
			Number:     5,
			Title:      "Change the branch identity to `test-aaaa` in the README.md file",
			Content:    "Signed-off-by: Akashdeep Dhar <testaddr@testaddr.com>",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "open",
			Created:    time.Date(2025, time.May, 5, 6, 43, 57, 0, time.UTC),
			Updated:    time.Date(2025, time.May, 19, 6, 29, 45, 0, time.UTC),
			Closed:     new(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)),
			MergedTime: new(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)),
			Merged:     false,
			PatchURL:   server.URL + "/protop2g-test-srce/pull-request/5.patch",
			Labels: []*base.Label{
				{
					Name: "aaaa",
				},
			},
			Head: base.PullRequestBranch{
				Ref:      "test-aaaa",
				SHA:      "b55e5c91d2572d60a8d7e71b3d3003e523127bd4",
				RepoName: "protop2g-test-srce",
				CloneURL: server.URL + "/protop2g-test-srce.git",
			},
			Base: base.PullRequestBranch{
				Ref:      "main",
				SHA:      "3f12d300f62f1c5b8a1d3265bd85d61cf6d924d7",
				RepoName: "protop2g-test-srce",
			},
		},
	}, prs)

	// Testing comments under pull requests
	comments, _, err = downloader.GetComments(prs[5])
	require.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex: 5,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.May, 5, 6, 44, 13, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Pull-request tagged with: aaaa",
		},
		{
			IssueIndex: 5,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.May, 7, 5, 25, 21, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "This is the first comment under this pull request.",
		},
		{
			IssueIndex: 5,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.May, 7, 5, 25, 29, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "This is the second comment under this pull request.",
		},
	}, comments)

	// Testing milestones migration
	milestones, err := downloader.GetMilestones()
	require.NoError(t, err)
	dict := map[string]*base.Milestone{
		"Milestone AAAA": {
			Title:    "Milestone AAAA",
			Deadline: new(time.Date(2025, time.December, 12, 0, 0, 0, 0, time.UTC)),
			State:    "open",
		},
		"Milestone BBBB": {
			Title:    "Milestone BBBB",
			Deadline: new(time.Date(2025, time.December, 12, 0, 0, 0, 0, time.UTC)),
			State:    "closed",
		},
		"Milestone CCCC": {
			Title:    "Milestone CCCC",
			Deadline: new(time.Date(2025, time.December, 12, 0, 0, 0, 0, time.UTC)),
			State:    "open",
		},
		"Milestone DDDD": {
			Title:    "Milestone DDDD",
			Deadline: new(time.Date(2025, time.December, 12, 0, 0, 0, 0, time.UTC)),
			State:    "closed",
		},
	}

	// We do not like when tests fail just because of dissimilar ordering
	for _, item := range milestones {
		assertMilestoneEqual(t, item, dict[item.Title])
	}
}

func TestPagureDownloadRepoWithPrivateIssues(t *testing.T) {
	t.Skip("Does not work")
	// Skip tests if Pagure token is not found
	cloneUser := os.Getenv("PAGURE_CLONE_USER")
	clonePassword := os.Getenv("PAGURE_CLONE_PASSWORD")
	apiUser := os.Getenv("PAGURE_API_USER")
	apiPassword := os.Getenv("PAGURE_API_TOKEN")

	fixtPath := "./testdata/pagure/full_download/authorized"
	server := unittest.NewMockWebServer(t, "https://pagure.io", fixtPath, false)
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	if clonePassword != "" || cloneUser != "" {
		serverURL.User = url.UserPassword(cloneUser, clonePassword)
	}
	serverURL.Path = "protop2g-test-srce.git"
	cloneAddr := serverURL.String()

	factory := &PagureDownloaderFactory{}
	downloader, err := factory.New(t.Context(), base.MigrateOptions{
		CloneAddr:    cloneAddr,
		AuthUsername: apiUser,
		AuthPassword: apiPassword,
		AuthToken:    apiPassword,
	})
	require.NoError(t, err)

	repo, err := downloader.GetRepoInfo()
	require.NoError(t, err)

	// Testing repository contents migration
	assertRepositoryEqual(t, &base.Repository{
		Name:        "protop2g-test-srce",
		Owner:       "",
		Description: "The source namespace for the Pagure Exporter project to run tests against",
		CloneURL:    cloneAddr,
		OriginalURL: strings.ReplaceAll(cloneAddr, ".git", ""),
	}, repo)

	topics, err := downloader.GetTopics()
	require.NoError(t, err)

	// Testing repository topics migration
	assert.Equal(t, []string{"srce", "test", "gridhead", "protop2g"}, topics)

	// Testing labels migration
	labels, err := downloader.GetLabels()
	require.NoError(t, err)
	assert.Len(t, labels, 15)

	// Testing issue tickets probing
	issues, isEnd, err := downloader.GetIssues(1, 20)
	require.NoError(t, err)
	assert.True(t, isEnd)

	// Testing issue tickets migration
	assertIssuesEqual(t, []*base.Issue{
		{
			Number:     4,
			Title:      "This is the title of the fourth test issue",
			Content:    "This is the body of the fourth test issue",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "closed",
			Milestone:  "Milestone DDDD",
			Created:    time.Date(2023, time.November, 21, 8, 6, 56, 0, time.UTC),
			Updated:    time.Date(2025, time.June, 25, 6, 26, 26, 0, time.UTC),
			Closed:     new(time.Date(2025, time.June, 25, 6, 23, 51, 0, time.UTC)),
			Labels: []*base.Label{
				{
					Name: "gggg",
				},
				{
					Name: "hhhh",
				},
				{
					Name: "Closed As/Baseless",
				},
				{
					Name: "Priority/Common",
				},
			},
		},
		{
			Number:     3,
			Title:      "This is the title of the third test issue",
			Content:    "This is the body of the third test issue",
			PosterName: "t0xic0der",
			PosterID:   -1,
			State:      "open",
			Milestone:  "Milestone CCCC",
			Created:    time.Date(2023, time.November, 21, 8, 3, 57, 0, time.UTC),
			Updated:    time.Date(2025, time.June, 25, 6, 26, 7, 0, time.UTC),
			Closed:     new(time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)),
			Labels: []*base.Label{
				{
					Name: "eeee",
				},
				{
					Name: "ffff",
				},
				{
					Name: "Priority/Uncommon",
				},
			},
		},
	}, issues)

	// Testing comments under issue tickets
	comments, _, err := downloader.GetComments(issues[0])
	require.NoError(t, err)
	assertCommentsEqual(t, []*base.Comment{
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2023, time.November, 21, 8, 7, 25, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "This is the first comment under the fourth test issue",
		},
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2023, time.November, 21, 8, 7, 34, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "This is the second comment under the fourth test issue",
		},
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2023, time.November, 21, 8, 8, 1, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue status updated to: Closed (was: Open)",
		},
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.May, 8, 4, 50, 46, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue set to the milestone: Milestone DDDD",
		},
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 23, 46, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue set to the milestone: None (was: Milestone DDDD)\n- Issue status updated to: Open (was: Closed)",
		},
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 23, 52, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue close_status updated to: Baseless\n- Issue status updated to: Closed (was: Open)",
		},
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 24, 55, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue set to the milestone: Milestone DDDD",
		},
		{
			IssueIndex: 4,
			PosterName: "t0xic0der",
			PosterID:   -1,
			Created:    time.Date(2025, time.June, 25, 6, 26, 26, 0, time.UTC),
			Updated:    time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
			Content:    "**Metadata Update from @t0xic0der**:\n- Issue priority set to: Common",
		},
	}, comments)
}

func TestProcessDate(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected time.Time
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: time.Time{},
		},
		{
			name:     "empty string",
			input:    new(""),
			expected: time.Time{},
		},
		{
			name:     "unix timestamp",
			input:    new("1609459200"),
			expected: time.Unix(1609459200, 0),
		},
		{
			name:     "YYYY-MM-DD format",
			input:    new("2025-12-12"),
			expected: time.Date(2025, time.December, 12, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processDate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
