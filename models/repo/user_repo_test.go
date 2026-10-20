// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoAssignees(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	users, err := repo_model.GetRepoAssignees(db.DefaultContext, repo2)
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, int64(2), users[0].ID)

	repo21 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 21})
	users, err = repo_model.GetRepoAssignees(db.DefaultContext, repo21)
	require.NoError(t, err)
	if assert.Len(t, users, 3) {
		assert.ElementsMatch(t, []int64{15, 16, 18}, []int64{users[0].ID, users[1].ID, users[2].ID})
	}

	// do not return deactivated users
	require.NoError(t, user_model.UpdateUserCols(db.DefaultContext, &user_model.User{ID: 15, IsActive: false}, "is_active"))
	users, err = repo_model.GetRepoAssignees(db.DefaultContext, repo21)
	require.NoError(t, err)
	if assert.Len(t, users, 2) {
		assert.NotContains(t, []int64{users[0].ID, users[1].ID}, 15)
	}
}

func assertReviewers(t *testing.T, repoID, posterID int64, expectedUIDs []int64) {
	ctx := db.DefaultContext
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})

	reviewers, err := repo_model.GetReviewers(ctx, repo, posterID)
	require.NoError(t, err)

	assert.Len(t, reviewers, len(expectedUIDs))
	for i, u := range reviewers {
		assert.Equal(t, expectedUIDs[i], u.ID)
	}
}

func TestRepoGetReviewers(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// public repository, including
	// - people with read access
	// - people watching pull requests
	// - members of the organization owning the repository
	// who are NOT the poster and active

	// repo 1 has 0 people with explicit access, 5 people watching PRs (including
	// user2 and one inactive user), owner user2
	assertReviewers(t, 1, 2, []int64{1, 11, 4})

	// poster which is a canidate reviewer
	assertReviewers(t, 1, 1, []int64{11, 4})

	// repo 23 has 2 people with explicit access, 0 people watching PRs, 4 people
	// being members of the org (including the two with access)
	assertReviewers(t, 23, 1, []int64{15, 18, 2, 29})

	// private repository, including
	// - people with read access
	// - the owner
	// who are NOT the poster and active

	// repo 6 has 0 people with explicit access, 0 people watching PRs, owned by
	// user 10
	assertReviewers(t, 6, 1, []int64{10})

	// repo 5 has 1 user with explicit access, 1 user watching PRs (without
	// access) 3 people with org access (including user 2), which will not be
	// counted
	assertReviewers(t, 5, 1, []int64{2})
}

func GetWatchedRepoIDsOwnedBy(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	repoIDs, err := repo_model.GetWatchedRepoIDsOwnedBy(db.DefaultContext, user1.ID, user2.ID)
	require.NoError(t, err)
	assert.Len(t, repoIDs, 1)
	assert.EqualValues(t, 1, repoIDs[0])
}
