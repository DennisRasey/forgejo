// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/log"
	base "forgejo.org/modules/migration"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/services/migrations/allowlist"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpload(t *testing.T) {
	unittest.PrepareTestEnv(t)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	var (
		opts = base.MigrateOptions{
			Issues: true,
		}
		repoName = "test_repo"
		uploader = NewGiteaLocalUploader(t.Context(), user, user.Name, repoName)
	)
	defer uploader.Close()

	fixturePath := "./testdata/github/full_download"
	server := unittest.NewMockWebServer(t, "https://api.github.com", fixturePath, false)
	defer server.Close()

	// Mock Data
	repoMock := &base.Repository{
		Name:          repoName,
		Owner:         "forgejo",
		Description:   "Some mock repo",
		CloneURL:      server.URL + "/forgejo/test_repo.git",
		OriginalURL:   server.URL + "/forgejo/test_repo",
		DefaultBranch: "master",
		Website:       "https://codeberg.org/forgejo/forgejo/",
	}

	// Create Repo
	require.NoError(t, uploader.CreateRepo(repoMock, opts))

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: repoName})

	t.Run("Topic", func(t *testing.T) {
		logChecker, cleanup := test.NewLogChecker(log.DEFAULT, log.INFO)
		logChecker.StopMark("Invalid topics: [verylongtopicthatistoolongforgejomigration a b c d e f] in migration to user1/test_repo")
		defer cleanup()

		require.NoError(t, uploader.CreateTopics("verylongtopicthatistoolongforgejomigration", "go  ", " security", "a b c d e f"))

		_, logStopped := logChecker.Check(time.Second)
		assert.True(t, logStopped)

		unittest.AssertExistsIf(t, false, &repo_model.Topic{Name: "verylongtopicthatistoolongforgejomigration"})
		unittest.AssertExistsIf(t, true, &repo_model.Topic{Name: "go"})
		unittest.AssertExistsIf(t, true, &repo_model.Topic{Name: "security"})
		unittest.AssertExistsIf(t, false, &repo_model.Topic{Name: "a b c d e f"})
	})

	t.Run("Issue", func(t *testing.T) {
		// Create and Test Issues Uploading
		issueA := &base.Issue{
			Title:        "First issue",
			Number:       0,
			PosterID:     37243484,
			PosterName:   "PatDyn",
			PosterEmail:  "",
			Content:      "Mock Content",
			Milestone:    "Mock Milestone",
			State:        "open",
			Created:      time.Date(2025, 8, 7, 12, 44, 7, 0, time.UTC),
			Updated:      time.Date(2025, 8, 7, 12, 44, 47, 0, time.UTC),
			Labels:       nil,
			Reactions:    nil,
			Closed:       nil,
			IsLocked:     false,
			Assignees:    nil,
			ForeignIndex: 0,
		}

		issueB := &base.Issue{
			Title:        "Second Issue",
			Number:       1,
			PosterID:     37243484,
			PosterName:   "PatDyn",
			PosterEmail:  "",
			Content:      "Mock Content",
			Milestone:    "Mock Milestone",
			State:        "open",
			Created:      time.Date(2025, 8, 7, 12, 45, 44, 0, time.UTC),
			Updated:      time.Date(2025, 8, 7, 13, 7, 25, 0, time.UTC),
			Labels:       nil,
			Reactions:    nil,
			Closed:       nil,
			IsLocked:     false,
			Assignees:    nil,
			ForeignIndex: 1,
		}

		err := uploader.CreateIssues(issueA, issueB)
		require.NoError(t, err)

		issues, err := issues_model.Issues(db.DefaultContext, &issues_model.IssuesOptions{
			RepoIDs:  []int64{repo.ID},
			IsPull:   optional.Some(false),
			SortType: "newest",
		})
		require.NoError(t, err)
		assert.Len(t, issues, 2)
	})

	t.Run("Comments", func(t *testing.T) {
		// Create and Test Comment Uploading
		issueAComment := &base.Comment{
			IssueIndex:  0,
			Index:       0,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			PosterEmail: "",
			Created:     time.Date(2025, 8, 7, 12, 44, 24, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 12, 44, 24, 0, time.UTC),
			Content:     "First Mock Comment",
			Reactions:   nil,
			Meta:        nil,
		}
		issueBComment := &base.Comment{
			IssueIndex:  1,
			Index:       1,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			PosterEmail: "",
			Created:     time.Date(2025, 8, 7, 13, 7, 25, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 13, 7, 25, 0, time.UTC),
			Content:     "Second Mock Comment",
			Reactions:   nil,
			Meta:        nil,
		}
		require.NoError(t, uploader.CreateComments(issueBComment, issueAComment))

		issues, err := issues_model.Issues(db.DefaultContext, &issues_model.IssuesOptions{
			RepoIDs:  []int64{repo.ID},
			IsPull:   optional.Some(false),
			SortType: "newest",
		})
		require.NoError(t, err)
		assert.Len(t, issues, 2)
		require.NoError(t, issues[0].LoadDiscussComments(db.DefaultContext))
		require.NoError(t, issues[1].LoadDiscussComments(db.DefaultContext))
		assert.Len(t, issues[0].Comments, 1)
		assert.Len(t, issues[1].Comments, 1)
	})

	t.Run("CommentReplies", func(t *testing.T) {
		find := func(issueIndex int64, suffix string) *issues_model.Comment {
			comments, err := issues_model.FindComments(db.DefaultContext, &issues_model.FindCommentsOptions{
				IssueID: uploader.issues[issueIndex].ID,
				Type:    issues_model.CommentTypeComment,
			})
			require.NoError(t, err)
			for _, comment := range comments {
				if strings.HasSuffix(comment.Content, suffix) {
					return comment
				}
			}
			require.FailNowf(t, "comment not found", "no comment of issue %d ends with %q", issueIndex, suffix)
			return nil
		}

		// Until EnableCommentReplyTo is called, Meta["ReplyTo"] is ignored: replies are
		// inserted untouched and no comment is tracked for later resolution.
		rawParent := &base.Comment{
			IssueIndex:  1,
			Index:       20,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 13, 58, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 13, 58, 0, 0, time.UTC),
			Content:     "Raw parent",
		}
		rawReply := &base.Comment{
			IssueIndex:  1,
			Index:       21,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 13, 59, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 13, 59, 0, 0, time.UTC),
			Content:     "Raw reply",
			Meta:        map[string]any{"ReplyTo": int64(20)},
		}
		require.NoError(t, uploader.CreateComments(rawParent, rawReply))
		assert.Equal(t, "Raw reply", find(1, "Raw reply").Content)
		assert.Nil(t, uploader.commentMap)

		uploader.EnableCommentReplyTo()

		// Once enabled, each reply gets a quote-reply header pointing to its own parent,
		// whether same or earlier batch; comments inserted before enabling stay untracked.
		parentA := &base.Comment{
			IssueIndex:  0,
			Index:       10,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 0, 0, 0, time.UTC),
			Content:     "Same batch parent",
		}
		sameBatchReply := &base.Comment{
			IssueIndex:  0,
			Index:       11,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 14, 1, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 1, 0, 0, time.UTC),
			Content:     "Same batch reply",
			Meta:        map[string]any{"ReplyTo": int64(10)},
		}
		parentB := &base.Comment{
			IssueIndex:  1,
			Index:       12,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 14, 2, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 2, 0, 0, time.UTC),
			Content:     "Cross batch parent",
		}
		require.NoError(t, uploader.CreateComments(parentA, sameBatchReply, parentB))

		crossBatchReply := &base.Comment{
			IssueIndex:  1,
			Index:       13,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 14, 3, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 3, 0, 0, time.UTC),
			Content:     "Cross batch reply",
			Meta:        map[string]any{"ReplyTo": int64(12)}, // "Cross batch parent", inserted by the previous batch
		}
		lateReply := &base.Comment{
			IssueIndex:  1,
			Index:       14,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 14, 4, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 4, 0, 0, time.UTC),
			Content:     "Late reply",
			Meta:        map[string]any{"ReplyTo": int64(20)}, // "Raw parent", inserted before enabling, untracked
		}
		require.NoError(t, uploader.CreateComments(crossBatchReply, lateReply))

		parentCommentA := find(0, "Same batch parent")
		assert.Equal(t, "Same batch parent", parentCommentA.Content)
		assert.Equal(t,
			fmt.Sprintf("@PatDyn wrote in %s/issues/%d#issuecomment-%d:\n> Same batch parent\n\nSame batch reply", repo.HTMLURL(), uploader.issues[0].Index, parentCommentA.ID),
			find(0, "Same batch reply").Content)

		parentCommentB := find(1, "Cross batch parent")
		assert.Equal(t, "Cross batch parent", parentCommentB.Content)
		assert.Equal(t,
			fmt.Sprintf("@PatDyn wrote in %s/issues/%d#issuecomment-%d:\n> Cross batch parent\n\nCross batch reply", repo.HTMLURL(), uploader.issues[1].Index, parentCommentB.ID),
			find(1, "Cross batch reply").Content)

		assert.Equal(t, "Late reply", find(1, "Late reply").Content)

		// A parent posted by a linked user (no OriginalAuthor) is attributed via its local
		// account; a mapping to a since-deleted user falls back to the Ghost user.
		uploader.userMap[601] = 2
		uploader.userMap[602] = unittest.NonexistentID
		linkedParent := &base.Comment{
			IssueIndex:  0,
			Index:       15,
			CommentType: "comment",
			PosterID:    601,
			PosterName:  "linked.user",
			Created:     time.Date(2025, 8, 7, 14, 5, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 5, 0, 0, time.UTC),
			Content:     "Linked parent",
		}
		ghostParent := &base.Comment{
			IssueIndex:  0,
			Index:       16,
			CommentType: "comment",
			PosterID:    602,
			PosterName:  "vanished.user",
			Created:     time.Date(2025, 8, 7, 14, 6, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 6, 0, 0, time.UTC),
			Content:     "Ghost parent",
		}
		linkedReply := &base.Comment{
			IssueIndex:  0,
			Index:       17,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 14, 7, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 7, 0, 0, time.UTC),
			Content:     "Linked reply",
			Meta:        map[string]any{"ReplyTo": int64(15)},
		}
		ghostReply := &base.Comment{
			IssueIndex:  0,
			Index:       18,
			CommentType: "comment",
			PosterID:    37243484,
			PosterName:  "PatDyn",
			Created:     time.Date(2025, 8, 7, 14, 8, 0, 0, time.UTC),
			Updated:     time.Date(2025, 8, 7, 14, 8, 0, 0, time.UTC),
			Content:     "Ghost reply",
			Meta:        map[string]any{"ReplyTo": int64(16)},
		}
		require.NoError(t, uploader.CreateComments(linkedParent, ghostParent, linkedReply, ghostReply))

		linkedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		assert.Equal(t,
			fmt.Sprintf("@%s wrote in %s/issues/%d#issuecomment-%d:\n> Linked parent\n\nLinked reply", linkedUser.Name, repo.HTMLURL(), uploader.issues[0].Index, find(0, "Linked parent").ID),
			find(0, "Linked reply").Content)
		assert.Equal(t,
			fmt.Sprintf("@%s wrote in %s/issues/%d#issuecomment-%d:\n> Ghost parent\n\nGhost reply", user_model.GhostUserName, repo.HTMLURL(), uploader.issues[0].Index, find(0, "Ghost parent").ID),
			find(0, "Ghost reply").Content)
	})

	// The mock server does not serve a clonable repository, so the migrated repository is
	// empty: craft the commit the pull request and its review will anchor to.
	var commit string
	t.Run("PullRequests", func(t *testing.T) {
		repoPath := repo.RepoPath()
		gitEnv := append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		blob, _, gitErr := git.NewCommand(t.Context(), "hash-object", "-w", "--stdin").RunStdString(&git.RunOpts{Dir: repoPath, Stdin: strings.NewReader("line 1\n")})
		require.NoError(t, gitErr)
		tree, _, gitErr := git.NewCommand(t.Context(), "mktree").RunStdString(&git.RunOpts{Dir: repoPath, Stdin: strings.NewReader("100644 blob " + strings.TrimSpace(blob) + "\treadme.md\n")})
		require.NoError(t, gitErr)
		commitOut, _, gitErr := git.NewCommand(t.Context(), "commit-tree", "-m", "Initial content").AddDynamicArguments(strings.TrimSpace(tree)).RunStdString(&git.RunOpts{Dir: repoPath, Env: gitEnv})
		require.NoError(t, gitErr)
		commit = strings.TrimSpace(commitOut)
		_, _, gitErr = git.NewCommand(t.Context(), "update-ref", "refs/heads/main").AddDynamicArguments(commit).RunStdString(&git.RunOpts{Dir: repoPath})
		require.NoError(t, gitErr)

		created := time.Date(2025, 8, 8, 10, 0, 0, 0, time.UTC)
		closed := time.Date(2025, 8, 8, 11, 0, 0, 0, time.UTC)
		pr := &base.PullRequest{
			Number:     3,
			Title:      "Mock PR",
			Content:    "Mock Content",
			PosterID:   37243484,
			PosterName: "PatDyn",
			State:      "closed",
			Created:    created,
			Updated:    closed,
			Closed:     &closed,
			Head: base.PullRequestBranch{
				Ref:       "pr-branch",
				SHA:       commit,
				RepoName:  repoName,
				OwnerName: user.Name,
			},
			Base: base.PullRequestBranch{
				Ref:       "main",
				RepoName:  repoName,
				OwnerName: user.Name,
			},
			EnsuredSafe: true,
		}
		// CreatePullRequests ends by enqueueing a patch-checking task, but the queue does not
		// exist in unit tests: go through its internals and skip the queue notification.
		gpr, err := uploader.newPullRequest(pr)
		require.NoError(t, err)
		require.NoError(t, uploader.remapUser(pr, gpr.Issue))
		require.NoError(t, issues_model.InsertPullRequests(db.DefaultContext, gpr))
		uploader.issues[gpr.Issue.Index] = gpr.Issue

		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Index: 3})
		assert.True(t, issue.IsPull)
		dbPR := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: issue.ID})
		// The merge base and the pull reference are the prerequisites of code review comments.
		assert.Equal(t, commit, dbPR.MergeBase)
		headCommitID, err := uploader.gitRepo.GetRefCommitID("refs/pull/3/head")
		require.NoError(t, err)
		assert.Equal(t, commit, headCommitID)
	})

	t.Run("Reviews", func(t *testing.T) {
		review := &base.Review{
			IssueIndex:   3,
			ReviewerID:   37243484,
			ReviewerName: "PatDyn",
			State:        base.ReviewStateCommented,
			CreatedAt:    time.Date(2025, 8, 8, 12, 0, 0, 0, time.UTC),
			Comments: []*base.ReviewComment{
				{
					// Shaped like the comments of the pre-existing downloaders: no PosterName
					// and no ExtraLinesCount.
					Content:   "Single line comment",
					TreePath:  "readme.md",
					Line:      1,
					DiffHunk:  "@@ -1 +1 @@",
					PosterID:  37243484,
					CreatedAt: time.Date(2025, 8, 8, 12, 0, 0, 0, time.UTC),
					UpdatedAt: time.Date(2025, 8, 8, 12, 0, 0, 0, time.UTC),
				},
				{
					// A comment spanning lines 1-3, written by someone else than the review author.
					Content:         "Range comment",
					TreePath:        "readme.md",
					Line:            1,
					ExtraLinesCount: 2,
					DiffHunk:        "@@ -1 +1 @@",
					PosterID:        1234,
					PosterName:      "OtherContributor",
					CreatedAt:       time.Date(2025, 8, 8, 12, 5, 0, 0, time.UTC),
					UpdatedAt:       time.Date(2025, 8, 8, 12, 5, 0, 0, time.UTC),
				},
			},
		}
		require.NoError(t, uploader.CreateReviews(review))

		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID, Index: 3})
		comments, err := issues_model.FindComments(db.DefaultContext, &issues_model.FindCommentsOptions{
			IssueID: issue.ID,
			Type:    issues_model.CommentTypeCode,
		})
		require.NoError(t, err)
		require.Len(t, comments, 2)
		byContent := map[string]*issues_model.Comment{}
		for _, comment := range comments {
			byContent[comment.Content] = comment
		}

		// A comment without a poster of its own keeps the review author and a single line,
		// like before ReviewComment carried PosterName and ExtraLinesCount.
		single := byContent["Single line comment"]
		require.NotNil(t, single)
		assert.Equal(t, "PatDyn", single.OriginalAuthor)
		assert.EqualValues(t, 1, single.Line)
		assert.EqualValues(t, 0, single.ExtraLinesCount)

		// A comment carrying its own poster and a range keeps both.
		ranged := byContent["Range comment"]
		require.NotNil(t, ranged)
		assert.Equal(t, "OtherContributor", ranged.OriginalAuthor)
		assert.EqualValues(t, 1, ranged.Line)
		assert.EqualValues(t, 2, ranged.ExtraLinesCount)
	})
}

func TestGiteaUploadRepo(t *testing.T) {
	// FIXME: Since no accesskey or user/password will trigger rate limit of github, just skip
	t.Skip()

	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	var (
		ctx        = t.Context()
		downloader = NewGithubDownloaderV3(ctx, "https://github.com", true, true, "", "", "", "go-xorm", "builder")
		repoName   = "builder-" + time.Now().Format("2006-01-02-15-04-05")
		uploader   = NewGiteaLocalUploader(t.Context(), user, user.Name, repoName)
	)

	err := migrateRepository(db.DefaultContext, user, downloader, uploader, base.MigrateOptions{
		CloneAddr:    "https://github.com/go-xorm/builder",
		RepoName:     repoName,
		AuthUsername: "",

		Wiki:         true,
		Issues:       true,
		Milestones:   true,
		Labels:       true,
		Releases:     true,
		Comments:     true,
		PullRequests: true,
		Private:      true,
		Mirror:       false,
	}, nil)
	require.NoError(t, err)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: repoName})
	assert.True(t, repo.HasWiki())
	assert.Equal(t, repo_model.RepositoryReady, repo.Status)
	assert.Empty(t, repo.Avatar)

	milestones, err := db.Find[issues_model.Milestone](db.DefaultContext, issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: optional.Some(false),
	})
	require.NoError(t, err)
	assert.Len(t, milestones, 1)

	milestones, err = db.Find[issues_model.Milestone](db.DefaultContext, issues_model.FindMilestoneOptions{
		RepoID:   repo.ID,
		IsClosed: optional.Some(true),
	})
	require.NoError(t, err)
	assert.Empty(t, milestones)

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, labels, 12)

	releases, err := db.Find[repo_model.Release](db.DefaultContext, repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: true,
		RepoID:      repo.ID,
	})
	require.NoError(t, err)
	assert.Len(t, releases, 8)

	releases, err = db.Find[repo_model.Release](db.DefaultContext, repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: false,
		RepoID:      repo.ID,
	})
	require.NoError(t, err)
	assert.Len(t, releases, 1)

	issues, err := issues_model.Issues(db.DefaultContext, &issues_model.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		IsPull:   optional.Some(false),
		SortType: "oldest",
	})
	require.NoError(t, err)
	assert.Len(t, issues, 15)
	require.NoError(t, issues[0].LoadDiscussComments(db.DefaultContext))
	assert.Empty(t, issues[0].Comments)

	pulls, _, err := issues_model.PullRequests(db.DefaultContext, repo.ID, &issues_model.PullRequestsOptions{
		SortType: "oldest",
	})
	require.NoError(t, err)
	assert.Len(t, pulls, 30)
	require.NoError(t, pulls[0].LoadIssue(db.DefaultContext))
	require.NoError(t, pulls[0].Issue.LoadDiscussComments(db.DefaultContext))
	assert.Len(t, pulls[0].Issue.Comments, 2)
}

func TestGiteaUploadRemapLocalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	repoName := "migrated"
	uploader := NewGiteaLocalUploader(t.Context(), doer, doer.Name, repoName)
	// call remapLocalUser
	uploader.sameApp = true

	externalID := int64(1234567)
	externalName := "username"
	source := base.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// The externalID does not match any existing user, everything
	// belongs to the Ghost user
	//
	target := repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err := uploader.remapUser(&source, &target)
	require.NoError(t, err)
	assert.EqualValues(t, user_model.GhostUserID, target.GetUserID())

	//
	// The externalID matches a known user but the name does not match,
	// everything belongs to the Ghost user
	//
	source.PublisherID = user.ID
	target = repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(&source, &target)
	require.NoError(t, err)
	assert.EqualValues(t, user_model.GhostUserID, target.GetUserID())

	//
	// The externalID and externalName match an existing user, everything
	// belongs to the existing user
	//
	source.PublisherName = user.Name
	target = repo_model.Release{}
	uploader.userMap = make(map[int64]int64)
	err = uploader.remapUser(&source, &target)
	require.NoError(t, err)
	assert.Equal(t, user.ID, target.GetUserID())
}

func TestGiteaUploadRemapExternalUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	repoName := "migrated"
	uploader := NewGiteaLocalUploader(t.Context(), doer, doer.Name, repoName)
	uploader.gitServiceType = structs.GiteaService
	// call remapExternalUser
	uploader.sameApp = false

	externalID := int64(1234567)
	externalName := "username"
	source := base.Release{
		PublisherID:   externalID,
		PublisherName: externalName,
	}

	//
	// When there is no user linked to the external ID, the migrated data is authored
	// by the Ghost user
	//
	uploader.userMap = make(map[int64]int64)
	target := repo_model.Release{}
	err := uploader.remapUser(&source, &target)
	require.NoError(t, err)
	assert.EqualValues(t, user_model.GhostUserID, target.GetUserID())

	//
	// Link the external ID to an existing user
	//
	linkedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	externalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    strconv.FormatInt(externalID, 10),
		UserID:        linkedUser.ID,
		LoginSourceID: 0,
		Provider:      structs.GiteaService.Name(),
	}
	err = user_model.LinkExternalToUser(db.DefaultContext, linkedUser, externalLoginUser)
	require.NoError(t, err)

	//
	// When a user is linked to the external ID, it becomes the author of
	// the migrated data
	//
	uploader.userMap = make(map[int64]int64)
	target = repo_model.Release{}
	err = uploader.remapUser(&source, &target)
	require.NoError(t, err)
	assert.Equal(t, linkedUser.ID, target.GetUserID())
}

func TestGiteaUploadUpdateGitForPullRequest(t *testing.T) {
	unittest.PrepareTestEnv(t)

	//
	// fromRepo master
	//
	fromRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	baseRef := "master"
	require.NoError(t, git.InitRepository(git.DefaultContext, fromRepo.RepoPath(), false, fromRepo.ObjectFormatName))
	err := git.NewCommand(git.DefaultContext, "symbolic-ref").AddDynamicArguments("HEAD", git.BranchPrefix+baseRef).Run(&git.RunOpts{Dir: fromRepo.RepoPath()})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fromRepo.RepoPath(), "README.md"), fmt.Appendf(nil, "# Testing Repository\n\nOriginally created in: %s", fromRepo.RepoPath()), 0o644))
	require.NoError(t, git.AddChanges(fromRepo.RepoPath(), true))
	signature := git.Signature{
		Email: "test@example.com",
		Name:  "test",
		When:  time.Now(),
	}
	require.NoError(t, git.CommitChanges(fromRepo.RepoPath(), git.CommitChangesOptions{
		Committer: &signature,
		Author:    &signature,
		Message:   "Initial Commit",
	}))
	fromGitRepo, err := gitrepo.OpenRepository(git.DefaultContext, fromRepo)
	require.NoError(t, err)
	defer fromGitRepo.Close()
	baseSHA, err := fromGitRepo.GetBranchCommitID(baseRef)
	require.NoError(t, err)

	//
	// fromRepo branch1
	//
	headRef := "branch1"
	_, _, err = git.NewCommand(git.DefaultContext, "checkout", "-b").AddDynamicArguments(headRef).RunStdString(&git.RunOpts{Dir: fromRepo.RepoPath()})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(fromRepo.RepoPath(), "README.md"), []byte("SOMETHING"), 0o644))
	require.NoError(t, git.AddChanges(fromRepo.RepoPath(), true))
	signature.When = time.Now()
	require.NoError(t, git.CommitChanges(fromRepo.RepoPath(), git.CommitChangesOptions{
		Committer: &signature,
		Author:    &signature,
		Message:   "Pull request",
	}))
	require.NoError(t, err)
	headSHA, err := fromGitRepo.GetBranchCommitID(headRef)
	require.NoError(t, err)

	fromRepoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: fromRepo.OwnerID})

	//
	// forkRepo branch2
	//
	forkHeadRef := "branch2"
	forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 8})
	require.NoError(t, git.CloneWithArgs(git.DefaultContext, nil, fromRepo.RepoPath(), forkRepo.RepoPath(), git.CloneRepoOptions{
		Branch: headRef,
	}))
	_, _, err = git.NewCommand(git.DefaultContext, "checkout", "-b").AddDynamicArguments(forkHeadRef).RunStdString(&git.RunOpts{Dir: forkRepo.RepoPath()})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(forkRepo.RepoPath(), "README.md"), fmt.Appendf(nil, "# branch2 %s", forkRepo.RepoPath()), 0o644))
	require.NoError(t, git.AddChanges(forkRepo.RepoPath(), true))
	require.NoError(t, git.CommitChanges(forkRepo.RepoPath(), git.CommitChangesOptions{
		Committer: &signature,
		Author:    &signature,
		Message:   "branch2 commit",
	}))
	forkGitRepo, err := gitrepo.OpenRepository(git.DefaultContext, forkRepo)
	require.NoError(t, err)
	defer forkGitRepo.Close()
	forkHeadSHA, err := forkGitRepo.GetBranchCommitID(forkHeadRef)
	require.NoError(t, err)

	toRepoName := "migrated"
	uploader := NewGiteaLocalUploader(t.Context(), fromRepoOwner, fromRepoOwner.Name, toRepoName)
	uploader.gitServiceType = structs.GiteaService
	require.NoError(t, uploader.CreateRepo(&base.Repository{
		Description: "description",
		OriginalURL: fromRepo.RepoPath(),
		CloneURL:    fromRepo.RepoPath(),
		IsPrivate:   false,
		IsMirror:    true,
	}, base.MigrateOptions{
		GitServiceType: structs.GiteaService,
		Private:        false,
		Mirror:         true,
	}))

	for _, testCase := range []struct {
		name        string
		head        string
		logFilter   []string
		logFiltered []bool
		pr          base.PullRequest
	}{
		{
			name: "fork, good Head.SHA",
			head: fmt.Sprintf("%s/%s", forkRepo.OwnerName, forkHeadRef),
			pr: base.PullRequest{
				Number: 1,
				State:  "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  forkRepo.RepoPath(),
					Ref:       forkHeadRef,
					SHA:       forkHeadSHA,
					RepoName:  forkRepo.Name,
					OwnerName: forkRepo.OwnerName,
				},
			},
		},
		{
			name: "fork, invalid Head.Ref",
			head: "unknown repository",
			pr: base.PullRequest{
				Number: 1,
				State:  "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  forkRepo.RepoPath(),
					Ref:       "INVALID",
					SHA:       forkHeadSHA,
					RepoName:  forkRepo.Name,
					OwnerName: forkRepo.OwnerName,
				},
			},
			logFilter:   []string{"Fetch branch from"},
			logFiltered: []bool{true},
		},
		{
			name: "invalid fork CloneURL",
			head: "unknown repository",
			pr: base.PullRequest{
				Number: 1,
				State:  "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  "UNLIKELY",
					Ref:       forkHeadRef,
					SHA:       forkHeadSHA,
					RepoName:  forkRepo.Name,
					OwnerName: "WRONG",
				},
			},
			logFilter:   []string{"AddRemote"},
			logFiltered: []bool{true},
		},
		{
			name: "no fork, good Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				Number: 1,
				State:  "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       headSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
		},
		{
			name: "no fork, empty Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				Number: 1,
				State:  "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       "",
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
			logFilter:   []string{"Empty reference", "Cannot remove local head"},
			logFiltered: []bool{true, false},
		},
		{
			name: "no fork, invalid Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				Number: 1,
				State:  "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       "brokenSHA",
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
			logFilter:   []string{"Deprecated local head"},
			logFiltered: []bool{true},
		},
		{
			name: "no fork, not found Head.SHA",
			head: headRef,
			pr: base.PullRequest{
				Number: 1,
				State:  "open",
				Base: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       baseRef,
					SHA:       baseSHA,
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
				Head: base.PullRequestBranch{
					CloneURL:  fromRepo.RepoPath(),
					Ref:       headRef,
					SHA:       "2697b352310fcd01cbd1f3dbd43b894080027f68",
					RepoName:  fromRepo.Name,
					OwnerName: fromRepo.OwnerName,
				},
			},
			logFilter:   []string{"Deprecated local head", "Cannot remove local head"},
			logFiltered: []bool{true, false},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stopMark := fmt.Sprintf(">>>>>>>>>>>>>STOP: %s<<<<<<<<<<<<<<<", testCase.name)

			logChecker, cleanup := test.NewLogChecker(log.DEFAULT, log.INFO)
			logChecker.Filter(testCase.logFilter...).StopMark(stopMark)
			defer cleanup()

			testCase.pr.EnsuredSafe = true

			head, err := uploader.updateGitForPullRequest(&testCase.pr)
			require.NoError(t, err)
			assert.Equal(t, testCase.head, head)

			log.Info(stopMark)

			logFiltered, logStopped := logChecker.Check(5 * time.Second)
			assert.True(t, logStopped)
			if len(testCase.logFilter) > 0 {
				assert.Equal(t, testCase.logFiltered, logFiltered, "for log message filters: %v", testCase.logFilter)
			}
		})
	}
}

func TestGiteaUploaderWithAvatar(t *testing.T) {
	defer test.MockVariableValueWithReset(&setting.Migrations.AllowLocalNetworks, true, func() { require.NoError(t, allowlist.Init()) })()
	unittest.PrepareTestEnv(t)
	defer test.MockVariableValue(&setting.Migrations.AvatarFetchTimeout, 1*time.Second)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	fixturePath := "./testdata/github/full_download"
	server := unittest.NewMockWebServer(t, "https://api.github.com", fixturePath, false)
	defer server.Close()

	blackPng, err := base64.URLEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQAAAAA3bvkkAAAACklEQVR4AWNgAAAAAgABc3UBGAAAAABJRU5ErkJggg==")
	if err != nil {
		t.Error(err)
		return
	}

	var tooWideBuf bytes.Buffer
	imgTooWide := image.NewGray(image.Rect(0, 0, 16001, 10))
	err = png.Encode(&tooWideBuf, imgTooWide)
	if err != nil {
		t.Error(err)
		return
	}
	imgTooWidePng := tooWideBuf.Bytes()

	var tooTallBuf bytes.Buffer
	imgTooTall := image.NewGray(image.Rect(0, 0, 10, 16002))
	err = png.Encode(&tooTallBuf, imgTooTall)
	if err != nil {
		t.Error(err)
		return
	}
	imgTooTallPng := tooTallBuf.Bytes()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/timeout":
			// Simulate a timeout by taking a long time to respond
			time.Sleep(8 * time.Second)
			w.Header().Set("Content-Type", "image/png")
			w.Write(blackPng)
		case "/not-found":
			http.NotFound(w, r)
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(blackPng)
		case "/weird-content":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
		case "/giant-response":
			w.Header().Set("Content-Type", "image/png")
			w.Write(make([]byte, 10485760))
		case "/invalid.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(make([]byte, 100))
		case "/mismatched.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(blackPng) // valid png, but wrong content-type
		case "/too-wide.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(imgTooWidePng)
		case "/too-tall.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(imgTooTallPng)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer imageServer.Close()

	for index, testCase := range []struct {
		name          string
		avatarURL     string
		expectSuccess bool
	}{
		{name: "avatar migration should succeed with valid URL", avatarURL: "/image.png", expectSuccess: true},
		{name: "avatar migration should fail with wrong content", avatarURL: "/weird-content", expectSuccess: false},
		{name: "avatar migration should fail with too many bytes", avatarURL: "/giant-response", expectSuccess: false},
		{name: "avatar migration should fail with invalid URL", avatarURL: "/not-found", expectSuccess: false},
		{name: "avatar migration should fail with invalid image", avatarURL: "/invalid.png", expectSuccess: false},
		{name: "avatar migration should fail with header and content mismatch", avatarURL: "/mismatched.jpg", expectSuccess: false},
		{name: "avatar migration should fail with timeout", avatarURL: "/timeout", expectSuccess: false},
		{name: "avatar migration should fail when too wide", avatarURL: "/too-wide.png", expectSuccess: false},
		{name: "avatar migration should fail when too tall", avatarURL: "/too-tall.png", expectSuccess: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repoName := "test_repo_" + strconv.Itoa(index)
			uploader := NewGiteaLocalUploader(t.Context(), user, user.Name, repoName)
			defer uploader.Close()

			// Mock Data
			repoMock := &base.Repository{
				Name:          repoName,
				Owner:         "forgejo",
				Description:   "Some mock repo",
				CloneURL:      server.URL + "/forgejo/test_repo.git",
				OriginalURL:   server.URL + "/forgejo/test_repo",
				DefaultBranch: "master",
				Website:       "https://codeberg.org/forgejo/forgejo/",
				AvatarURL:     imageServer.URL + testCase.avatarURL,
			}

			// regardless of whether the avatar migration succeeds, the overall migration should succeed
			require.NoError(t, uploader.CreateRepo(repoMock, MigrateOptions{}))

			if testCase.expectSuccess {
				assert.NotEmpty(t, uploader.repo.Avatar)
			} else {
				assert.Empty(t, uploader.repo.Avatar)
			}
		})
	}
}
