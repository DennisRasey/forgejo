// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"testing"

	migration_tests "forgejo.org/models/gitea_migrations/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RemoveInvalidLabels(t *testing.T) {
	// Models used by the migration
	type Comment struct {
		ID           int64 `xorm:"pk autoincr"`
		Type         int   `xorm:"INDEX"`
		IssueID      int64 `xorm:"INDEX"`
		LabelID      int64
		ShouldRemain bool // <- Flag for testing the migration
	}

	type Issue struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"INDEX UNIQUE(repo_index)"`
		Index  int64 `xorm:"UNIQUE(repo_index)"` // Index in one repository.
	}

	type Repository struct {
		ID        int64  `xorm:"pk autoincr"`
		OwnerID   int64  `xorm:"UNIQUE(s) index"`
		LowerName string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	}

	type Label struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"INDEX"`
		OrgID  int64 `xorm:"INDEX"`
	}

	type IssueLabel struct {
		ID           int64 `xorm:"pk autoincr"`
		IssueID      int64 `xorm:"UNIQUE(s)"`
		LabelID      int64 `xorm:"UNIQUE(s)"`
		ShouldRemain bool  // <- Flag for testing the migration
	}

	// load and prepare the test database
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(Comment), new(Issue), new(Repository), new(IssueLabel), new(Label))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	var issueLabels []*IssueLabel
	ilPreMigration := map[int64]*IssueLabel{}
	ilPostMigration := map[int64]*IssueLabel{}

	var comments []*Comment
	comPreMigration := map[int64]*Comment{}
	comPostMigration := map[int64]*Comment{}

	// Get pre migration values
	err := x.Find(&issueLabels)
	require.NoError(t, err, "Unable to find issueLabels")

	for _, issueLabel := range issueLabels {
		ilPreMigration[issueLabel.ID] = issueLabel
	}

	err = x.Find(&comments)
	require.NoError(t, err, "Unable to find comments")

	for _, comment := range comments {
		comPreMigration[comment.ID] = comment
	}

	// Run the migration
	err = RemoveInvalidLabels(x)
	require.NoError(t, err, "unable to RemoveInvalidLabels")

	// Get the post migration values
	issueLabels = issueLabels[:0]
	err = x.Find(&issueLabels)
	require.NoError(t, err, "Unable to find issueLabels")

	for _, issueLabel := range issueLabels {
		ilPostMigration[issueLabel.ID] = issueLabel
	}

	comments = comments[:0]
	err = x.Find(&comments)
	require.NoError(t, err, "Unable to find comments")

	for _, comment := range comments {
		comPostMigration[comment.ID] = comment
	}

	// Finally test results of the migration
	for id, comment := range comPreMigration {
		post, ok := comPostMigration[id]
		if ok {
			assert.True(t, comment.ShouldRemain, "Comment[%d] remained but should have been deleted", id)
			assert.Equal(t, comment, post)
		} else if comment.ShouldRemain {
			t.Errorf("Comment[%d] was deleted but should have remained", id)
		}
	}

	for id, il := range ilPreMigration {
		post, ok := ilPostMigration[id]
		if ok {
			assert.True(t, il.ShouldRemain, "IssueLabel[%d] remained but should have been deleted", id)
			assert.Equal(t, il, post)
		} else if il.ShouldRemain {
			t.Errorf("IssueLabel[%d] was deleted but should have remained", id)
		}
	}
}
