// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package convert

import (
	"context"
	"fmt"
	"testing"

	activities_model "forgejo.org/models/activities"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/forgefed"
	"forgejo.org/modules/json"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertConvertedContains(ctx context.Context, t *testing.T, a *activities_model.Action, text []string) {
	act, err := ActionToForgeUserActivity(ctx, a)
	require.NoError(t, err)

	note := act.Object.(forgefed.ForgeUserActivityNote)
	noteContent := note.Content.Get(ap.NilLangRef).String()

	for _, s := range text {
		assert.Contains(t, noteContent, s)
	}
}

func TestActionToForgeUserActivity(t *testing.T) {
	require.NoError(t, unittest.LoadFixtures())
	ctx := t.Context()

	// invalid repo
	unittest.AssertNotExistsBean(t, &repo_model.Repository{ID: 9999})
	a := &activities_model.Action{
		RepoID: 9999,
	}

	_, err := ActionToForgeUserActivity(ctx, a)
	require.ErrorIs(t, err, repo_model.ErrRepoNotExist{})

	// render repo from nonexistent user (renderRepo)
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	unittest.AssertNotExistsBean(t, &user_model.User{ID: 9999})
	a = &activities_model.Action{
		RepoID:    1,
		ActUserID: 9999,
		OpType:    activities_model.ActionCreateRepo,
	}

	assertConvertedContains(ctx, t, a, []string{"Ghost", "created a new repository", "repo1"})

	// push commits from existing user (renderCommit, renderBranch, renderRepo)
	unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	a = &activities_model.Action{
		RepoID:    1,
		ActUserID: 5,
		OpType:    activities_model.ActionCommitRepo,
		RefName:   "refs/head/meow",
		Content:   `{"len": 2, "commits": [{"message": "uwu", "sha1": "1337"}, {"message": "nya", "sha1": "2342"}]}`,
	}

	assertConvertedContains(ctx, t, a, []string{"user5", "pushed to", "meow"})

	// open issue frome xisting user (renderIssue)
	issue := issues_model.Issue{ID: 1}
	unittest.AssertExistsAndLoadBean(t, &issue)
	issueMarshal, err := json.Marshal([]string{fmt.Sprintf("%d", issue.Index), issue.Title})
	require.NoError(t, err)
	a = &activities_model.Action{
		RepoID:    1,
		ActUserID: 5,
		OpType:    activities_model.ActionCreateIssue,
		Content:   string(issueMarshal),
	}

	assertConvertedContains(ctx, t, a, []string{"user5", "opened issue", "repo1/issues/1"})

	// push a tag (renderTag)
	a = &activities_model.Action{
		RepoID:    1,
		ActUserID: 5,
		OpType:    activities_model.ActionPushTag,
		RefName:   "refs/tags/uwu",
		Content:   `{"len": 2, "commits": [{"message": "uwu", "sha1": "1337"}, {"message": "nya", "sha1": "2342"}]}`,
	}

	assertConvertedContains(ctx, t, a, []string{"user5", "pushed", "uwu", "repo1"})

	// comment on an issue (markdown rendering)
	comment := issues_model.Issue{ID: 1}
	unittest.AssertExistsAndLoadBean(t, &comment)
	a = &activities_model.Action{
		RepoID:    1,
		ActUserID: 5,
		OpType:    activities_model.ActionCommentIssue,
		CommentID: 1,
		Content:   string(issueMarshal),
	}

	assertConvertedContains(ctx, t, a, []string{"user5", "commented", "repo1/issues/1", "<p>1</p>"})
}
