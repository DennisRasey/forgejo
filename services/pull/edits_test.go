// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"testing"

	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckHeadBranchEditable(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	other := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	openPR := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	mergedPR := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})

	t.Run("owner on an open pull request", func(t *testing.T) {
		require.NoError(t, CheckHeadBranchEditable(db.DefaultContext, owner, openPR))
		ok, err := CanEditHeadBranch(db.DefaultContext, owner, openPR)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("anonymous", func(t *testing.T) {
		require.ErrorIs(t, CheckHeadBranchEditable(db.DefaultContext, nil, openPR), util.ErrPermissionDenied)
		ok, err := CanEditHeadBranch(db.DefaultContext, nil, openPR)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("user without write access", func(t *testing.T) {
		require.ErrorIs(t, CheckHeadBranchEditable(db.DefaultContext, other, openPR), util.ErrPermissionDenied)
	})

	t.Run("merged pull request", func(t *testing.T) {
		require.ErrorIs(t, CheckHeadBranchEditable(db.DefaultContext, owner, mergedPR), ErrHeadBranchNotEditable)
		ok, err := CanEditHeadBranch(db.DefaultContext, owner, mergedPR)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("agit flow", func(t *testing.T) {
		agitPR := *openPR
		agitPR.Flow = issues_model.PullRequestFlowAGit
		require.ErrorIs(t, CheckHeadBranchEditable(db.DefaultContext, owner, &agitPR), ErrHeadBranchNotEditable)
	})
}
