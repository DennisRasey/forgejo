// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"forgejo.org/models/unittest"
	base "forgejo.org/modules/migration"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/services/migrations/allowlist"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBitbucketDataCenterURL(t *testing.T) {
	cases := []struct {
		url     string
		base    string
		project string
		repo    string
	}{
		{"https://bitbucket.example.com/scm/PROJ/myrepo.git", "https://bitbucket.example.com", "PROJ", "myrepo"},
		{"https://bitbucket.example.com/context/scm/PROJ/myrepo.git", "https://bitbucket.example.com/context", "PROJ", "myrepo"},
	}
	for _, c := range cases {
		u, err := url.Parse(c.url)
		require.NoError(t, err)
		baseURL, project, repo, err := parseBitbucketDataCenterURL(u)
		require.NoError(t, err, c.url)
		assert.Equal(t, c.base, baseURL.String(), c.url)
		assert.Equal(t, c.project, project, c.url)
		assert.Equal(t, c.repo, repo, c.url)
	}

	for _, invalid := range []string{
		"https://bitbucket.example.com/unexpected/path",
		"https://bitbucket.example.com/projects/PROJ/repos/myrepo/browse",
		"https://bitbucket.example.com/scm/onlyproject",
	} {
		u, err := url.Parse(invalid)
		require.NoError(t, err)
		_, _, _, err = parseBitbucketDataCenterURL(u)
		require.Error(t, err, invalid)
	}
}

func TestBitbucketDataCenterFormatCloneURL(t *testing.T) {
	d := &BitbucketDataCenterDownloader{}

	got, err := d.FormatCloneURL(base.MigrateOptions{AuthToken: "sometoken"}, "https://bitbucket.example.com/scm/PROJ/myrepo.git")
	require.NoError(t, err)
	assert.Equal(t, "https://x-token-auth:sometoken@bitbucket.example.com/scm/PROJ/myrepo.git", got)

	got, err = d.FormatCloneURL(base.MigrateOptions{AuthUsername: "jdoe", AuthToken: "sometoken"}, "https://bitbucket.example.com/scm/PROJ/myrepo.git")
	require.NoError(t, err)
	assert.Equal(t, "https://jdoe:sometoken@bitbucket.example.com/scm/PROJ/myrepo.git", got)

	got, err = d.FormatCloneURL(base.MigrateOptions{AuthUsername: "jdoe", AuthPassword: "secret"}, "https://bitbucket.example.com/scm/PROJ/myrepo.git")
	require.NoError(t, err)
	assert.Equal(t, "https://jdoe:secret@bitbucket.example.com/scm/PROJ/myrepo.git", got)

	got, err = d.FormatCloneURL(base.MigrateOptions{}, "https://bitbucket.example.com/scm/PROJ/myrepo.git")
	require.NoError(t, err)
	assert.Equal(t, "https://bitbucket.example.com/scm/PROJ/myrepo.git", got)
}

func TestBitbucketDataCenterDownloaderBlocksLocalhost(t *testing.T) {
	defer test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, false, func() { require.NoError(t, allowlist.Init()) })()

	factory := &BitbucketDataCenterDownloaderFactory{}
	downloader, err := factory.New(t.Context(), base.MigrateOptions{CloneAddr: "http://localhost/scm/PROJ/myrepo.git"})
	require.NoError(t, err)

	_, err = downloader.GetRepoInfo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can only call allowed HTTP servers")
}

// TestBitbucketDataCenterAuthorizationHeader pins the exact Authorization header sent to the REST API.
func TestBitbucketDataCenterAuthorizationHeader(t *testing.T) {
	t.Cleanup(test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, true, func() { require.NoError(t, allowlist.Init()) }))

	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"slug":"test-repo","links":{"clone":[{"name":"http","href":"http://bitbucket.example.com/scm/migr/test-repo.git"}]}}`))
	}))
	t.Cleanup(server.Close)

	factory := &BitbucketDataCenterDownloaderFactory{}
	downloader, err := factory.New(t.Context(), base.MigrateOptions{
		CloneAddr: server.URL + "/scm/migr/test-repo.git",
		AuthToken: "sometoken",
	})
	require.NoError(t, err)

	_, err = downloader.GetRepoInfo()
	require.NoError(t, err)
	assert.Equal(t, "Bearer sometoken", gotAuthorization)
}

// newBitbucketDataCenterFixtureDownloader returns a downloader backed by the fixtures of testdata/bitbucketdc
func newBitbucketDataCenterFixtureDownloader(t *testing.T) (base.Downloader, string) {
	t.Cleanup(test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, true, func() { require.NoError(t, allowlist.Init()) }))

	liveURL := os.Getenv("BITBUCKET_DC_URL")
	token := os.Getenv("BITBUCKET_DC_TOKEN")
	liveMode := liveURL != "" && token != ""
	baseURL := "https://bitbucket.example.com"
	if liveMode {
		baseURL = liveURL
	}
	fixturePath := "./testdata/bitbucketdc/full_download"
	if !liveMode {
		if entries, err := os.ReadDir(fixturePath); err != nil || len(entries) == 0 {
			t.Skip("fixtures not recorded, set BITBUCKET_DC_URL and BITBUCKET_DC_TOKEN to record them")
		}
	}
	server := unittest.NewMockWebServer(t, baseURL, fixturePath, liveMode)
	t.Cleanup(server.Close)

	factory := &BitbucketDataCenterDownloaderFactory{}
	downloader, err := factory.New(t.Context(), base.MigrateOptions{
		CloneAddr: server.URL + "/scm/migr/test-repo.git",
		AuthToken: token,
	})
	require.NoError(t, err)
	return downloader, server.URL
}

func TestBitbucketDataCenterDownloadRepo(t *testing.T) {
	downloader, serverURL := newBitbucketDataCenterFixtureDownloader(t)

	assert.True(t, downloader.SupportCommentReplyTo())

	repo, err := downloader.GetRepoInfo()
	require.NoError(t, err)
	assert.Equal(t, "test-repo", repo.Name)
	assert.Equal(t, "migr", repo.Owner)
	assert.Equal(t, "main", repo.DefaultBranch)
	assert.True(t, repo.IsPrivate)
	assert.Equal(t, serverURL+"/scm/migr/test-repo.git", repo.CloneURL)
	assert.Equal(t, serverURL+"/projects/MIGR/repos/test-repo/browse", repo.OriginalURL)

	// Four pull requests fetched two by two: the cursor pagination runs against real responses.
	prPage, isEnd, err := downloader.GetPullRequests(1, 2)
	require.NoError(t, err)
	assert.False(t, isEnd)
	require.Len(t, prPage, 2)
	prs := map[int64]*base.PullRequest{prPage[0].Number: prPage[0], prPage[1].Number: prPage[1]}
	prPage, isEnd, err = downloader.GetPullRequests(2, 2)
	require.NoError(t, err)
	assert.True(t, isEnd)
	require.Len(t, prPage, 2)
	prs[prPage[0].Number] = prPage[0]
	prs[prPage[1].Number] = prPage[1]
	require.Len(t, prs, 4)

	open := prs[1]
	assert.Equal(t, "First feature", open.Title)
	assert.Equal(t, "Description of First feature", open.Content)
	assert.Equal(t, "open", open.State)
	assert.False(t, open.Merged)
	assert.Nil(t, open.Closed)
	assert.Equal(t, "feature/one", open.Head.Ref)
	assert.Equal(t, "main", open.Base.Ref)
	assert.Len(t, open.Head.SHA, 40)
	assert.NotEmpty(t, open.PosterName)
	assert.Positive(t, open.PosterID)
	assert.False(t, open.IsForkPullRequest())
	assert.True(t, open.EnsuredSafe)

	merged := prs[2]
	assert.Equal(t, "Second feature", merged.Title)
	assert.Equal(t, "closed", merged.State)
	assert.True(t, merged.Merged)
	assert.NotNil(t, merged.MergedTime)
	assert.NotNil(t, merged.Closed)

	declined := prs[3]
	assert.Equal(t, "Third feature", declined.Title)
	assert.Equal(t, "closed", declined.State)
	assert.False(t, declined.Merged)
	assert.NotNil(t, declined.Closed)

	// General comments of the open pull request, in chronological order.
	comments, _, err := downloader.GetComments(open)
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, int64(1), comments[0].IssueIndex)
	assert.Equal(t, "A general question", comments[0].Content)
	assert.Equal(t, "A general answer", comments[1].Content)
	assert.NotEqual(t, comments[0].PosterName, comments[1].PosterName)
	assert.LessOrEqual(t, comments[0].Created.UnixMilli(), comments[1].Created.UnixMilli())
	// The answer is a reply: it references its parent so the uploader links them.
	assert.Nil(t, comments[0].Meta)
	assert.Equal(t, comments[0].Index, comments[1].Meta["ReplyTo"])

	// Reviews of the open pull request:
	// an inline thread on lines 2-4
	// a comment on the old side of the diff
	// a "needs work"
	reviews, err := downloader.GetReviews(open)
	require.NoError(t, err)
	require.Len(t, reviews, 3)
	for _, review := range reviews {
		assert.NotEqual(t, base.ReviewStateApproved, review.State)
	}

	thread := reviews[0]
	assert.Equal(t, base.ReviewStateCommented, thread.State)
	require.Len(t, thread.Comments, 2)
	assert.Equal(t, "This block needs a rewrite", thread.Comments[0].Content)
	assert.Equal(t, "Rewritten in the next push", thread.Comments[1].Content)
	assert.Equal(t, "README.md", thread.Comments[0].TreePath)
	assert.Equal(t, 2, thread.Comments[0].Line)
	assert.Equal(t, int64(2), thread.Comments[0].ExtraLinesCount)
	assert.NotEmpty(t, thread.Comments[0].DiffHunk)
	assert.NotEqual(t, thread.Comments[0].PosterName, thread.Comments[1].PosterName)

	oldSide := reviews[1]
	assert.Equal(t, base.ReviewStateCommented, oldSide.State)
	require.Len(t, oldSide.Comments, 1)
	assert.Equal(t, "Why was this removed?", oldSide.Comments[0].Content)
	assert.Equal(t, "README.md", oldSide.Comments[0].TreePath)
	assert.Equal(t, -2, oldSide.Comments[0].Line)
	assert.Equal(t, int64(0), oldSide.Comments[0].ExtraLinesCount)

	needsWork := reviews[2]
	assert.Equal(t, base.ReviewStateChangesRequested, needsWork.State)
	assert.NotEmpty(t, needsWork.ReviewerName)
	assert.NotEqual(t, open.PosterName, needsWork.ReviewerName)

	// The merged and declined pull requests carry no discussion.
	comments, _, err = downloader.GetComments(merged)
	require.NoError(t, err)
	assert.Empty(t, comments)
	reviews, err = downloader.GetReviews(declined)
	require.NoError(t, err)
	assert.Empty(t, reviews)
}

// TestBitbucketDataCenterDownloadForkPullRequest replays the same fixtures and focuses on the
// pull request opened from a fork in the author's personal project
func TestBitbucketDataCenterDownloadForkPullRequest(t *testing.T) {
	downloader, _ := newBitbucketDataCenterFixtureDownloader(t)

	prs, _, err := downloader.GetPullRequests(1, 2)
	require.NoError(t, err)
	page, _, err := downloader.GetPullRequests(2, 2)
	require.NoError(t, err)
	prs = append(prs, page...)

	var forkPR *base.PullRequest
	for _, pr := range prs {
		if pr.Number == 4 {
			forkPR = pr
		}
	}
	require.NotNil(t, forkPR)

	assert.Equal(t, "Fork feature", forkPR.Title)
	assert.Equal(t, "open", forkPR.State)
	assert.True(t, forkPR.IsForkPullRequest())
	assert.Equal(t, "feature/fork", forkPR.Head.Ref)
	assert.Equal(t, "test-repo", forkPR.Head.RepoName)
	assert.True(t, strings.HasPrefix(forkPR.Head.OwnerName, "~"))
	assert.Contains(t, forkPR.Head.CloneURL, "/scm/~")
	assert.True(t, strings.HasSuffix(forkPR.Head.CloneURL, "/test-repo.git"))
	assert.True(t, forkPR.EnsuredSafe)
}

// TestBitbucketDataCenterActivityDeduplication covers with a synthetic response the branch a
// recorded fixture cannot deterministically produce: a comment surfacing both as its own
// activity and nested in the snapshot of its thread must be kept once.
func TestBitbucketDataCenterActivityDeduplication(t *testing.T) {
	defer test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, true, func() { require.NoError(t, allowlist.Init()) })()

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/myrepo/pull-requests", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"isLastPage": true,
			"values": [
				{
					"id": 1,
					"title": "A commented pull request",
					"state": "OPEN",
					"createdDate": 1704067200000,
					"updatedDate": 1704153600000,
					"fromRef": {
						"displayId": "fix",
						"latestCommit": "0123456789012345678901234567890123456789",
						"repository": {"slug": "myrepo", "project": {"key": "PROJ"}}
					},
					"toRef": {
						"displayId": "main",
						"latestCommit": "aaaa456789012345678901234567890123456789",
						"repository": {"slug": "myrepo", "project": {"key": "PROJ"}}
					},
					"author": {"user": {"id": 9, "name": "alice", "emailAddress": "alice@example.com"}}
				}
			]
		}`)
	})
	// The reply surfaces both nested in its thread and as its own newer activity.
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/myrepo/pull-requests/1/activities", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"isLastPage": true,
			"values": [
				{
					"id": 102,
					"action": "COMMENTED",
					"commentAction": "ADDED",
					"createdDate": 1704260000000,
					"user": {"id": 7, "name": "bob"},
					"comment": {
						"id": 21,
						"text": "A reply",
						"author": {"id": 7, "name": "bob", "emailAddress": "bob@example.com"},
						"createdDate": 1704260000000,
						"updatedDate": 1704260000000
					}
				},
				{
					"id": 101,
					"action": "COMMENTED",
					"commentAction": "ADDED",
					"createdDate": 1704250000000,
					"user": {"id": 9, "name": "alice"},
					"comment": {
						"id": 20,
						"text": "A question",
						"author": {"id": 9, "name": "alice", "emailAddress": "alice@example.com"},
						"createdDate": 1704250000000,
						"updatedDate": 1704250000000,
						"comments": [
							{
								"id": 21,
								"text": "A reply",
								"author": {"id": 7, "name": "bob", "emailAddress": "bob@example.com"},
								"createdDate": 1704260000000,
								"updatedDate": 1704260000000
							}
						]
					}
				}
			]
		}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	factory := &BitbucketDataCenterDownloaderFactory{}
	downloader, err := factory.New(t.Context(), base.MigrateOptions{
		CloneAddr: server.URL + "/scm/PROJ/myrepo.git",
	})
	require.NoError(t, err)

	prs, isEnd, err := downloader.GetPullRequests(1, 10)
	require.NoError(t, err)
	assert.True(t, isEnd)
	require.Len(t, prs, 1)

	// The duplicated reply is kept once, chronologically after its parent.
	comments, _, err := downloader.GetComments(prs[0])
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, "A question", comments[0].Content)
	assert.Equal(t, "A reply", comments[1].Content)
	assert.Equal(t, "bob", comments[1].PosterName)
	assert.Equal(t, comments[0].Index, comments[1].Meta["ReplyTo"])
}

func TestBitbucketDataCenterGetRepoInfoWithoutHTTPCloneLink(t *testing.T) {
	defer test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, true, func() { require.NoError(t, allowlist.Init()) })()

	// When HTTP(S) SCM hosting is disabled, the API only exposes the ssh clone link.
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/projects/PROJ/repos/myrepo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"slug": "myrepo",
			"links": {"clone": [{"href": "ssh://git@bitbucket.example.com:7999/PROJ/myrepo.git", "name": "ssh"}]}
		}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	factory := &BitbucketDataCenterDownloaderFactory{}
	downloader, err := factory.New(t.Context(), base.MigrateOptions{
		CloneAddr: server.URL + "/scm/PROJ/myrepo.git",
	})
	require.NoError(t, err)

	_, err = downloader.GetRepoInfo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP(S) SCM hosting")
}
