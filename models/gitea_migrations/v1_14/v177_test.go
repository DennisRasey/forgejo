// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"testing"

	migration_tests "forgejo.org/models/gitea_migrations/test"
	"forgejo.org/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DeleteOrphanedIssueLabels(t *testing.T) {
	// Create the models used in the migration
	type IssueLabel struct {
		ID      int64 `xorm:"pk autoincr"`
		IssueID int64 `xorm:"UNIQUE(s)"`
		LabelID int64 `xorm:"UNIQUE(s)"`
	}

	type Label struct {
		ID              int64 `xorm:"pk autoincr"`
		RepoID          int64 `xorm:"INDEX"`
		OrgID           int64 `xorm:"INDEX"`
		Name            string
		Description     string
		Color           string `xorm:"VARCHAR(7)"`
		NumIssues       int
		NumClosedIssues int
		CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	// Prepare and load the testing database
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(IssueLabel), new(Label))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	var issueLabels []*IssueLabel
	preMigration := map[int64]*IssueLabel{}
	postMigration := map[int64]*IssueLabel{}

	// Load issue labels that exist in the database pre-migration
	err := x.Find(&issueLabels)
	require.NoError(t, err)

	for _, issueLabel := range issueLabels {
		preMigration[issueLabel.ID] = issueLabel
	}

	// Run the migration
	err = DeleteOrphanedIssueLabels(x)
	require.NoError(t, err)

	// Load the remaining issue-labels
	issueLabels = issueLabels[:0]
	err = x.Find(&issueLabels)
	require.NoError(t, err)

	for _, issueLabel := range issueLabels {
		postMigration[issueLabel.ID] = issueLabel
	}

	// Now test what is left
	_, ok := postMigration[2]
	require.False(t, ok, "Orphaned Label[2] survived the migration")

	_, ok = postMigration[5]
	require.False(t, ok, "Orphaned Label[5] survived the migration")

	for id, post := range postMigration {
		pre := preMigration[id]
		assert.Equal(t, pre, post, "migration changed issueLabel %d", id)
	}
}
