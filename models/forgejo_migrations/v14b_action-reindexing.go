// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
	"code.forgejo.org/xorm/xorm/schemas"
)

func init() {
	registerMigration(&Migration{
		Description: "rework indexes on table action",
		Upgrade:     reworkActionIndexes,
	})
}

type v14bAction struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64 // Receiver user id.
	OpType      int
	ActUserID   int64 // Action user id.
	RepoID      int64
	CommentID   int64 `xorm:"INDEX"`         // indexed to support `DeleteIssueActions`
	CreatedUnix int64 `xorm:"created INDEX"` // indexed to support `DeleteOldActions`
}

// TableName sets the name of this table
func (a *v14bAction) TableName() string {
	return "action"
}

// TableIndices implements xorm's TableIndices interface.  It is used here to ensure indexes with specified column order
// are created, which can't be created through xorm tags on the struct.
func (a *v14bAction) TableIndices() []*schemas.Index {
	// Index to support getUserHeatmapData, which searches for data that is visible-to (user_id) and performed-by
	// (act_user_id) a user, but only includes visible repos (repo_id).
	actUserIndex := schemas.NewIndex("au_r_c_u", schemas.IndexType)
	actUserIndex.AddColumn("act_user_id", "repo_id", "created_unix", "user_id")

	// GetFeeds is a common access point to Action and requires that all action feeds be queried based upon one of
	// user_id (opts.RequestedUser), repo_id (opts.RequestedTeam... kinda), and/or repo_id (opts.RequestedRepo), and
	// then the results are ordered by created_unix and paginated.  The most efficient indexes to support those queries
	// are:
	requestedUser := schemas.NewIndex("user_id_created_unix", schemas.IndexType)
	requestedUser.AddColumn("user_id", "created_unix")
	requestedRepo := schemas.NewIndex("repo_id_created_unix", schemas.IndexType)
	requestedRepo.AddColumn("repo_id", "created_unix")

	// To support `DeleteIssueActions` search for createissue / createpullrequest actions; this isn't a great search
	// because `DeleteIssueActions` searches by `content` as well, but it should be sufficient performance-wise for
	// infrequent deleting of issues.
	repoOpType := schemas.NewIndex("repo_id_op_type", schemas.IndexType)
	repoOpType.AddColumn("repo_id", "op_type")

	indices := []*schemas.Index{actUserIndex, requestedUser, requestedRepo, repoOpType}

	return indices
}

func reworkActionIndexes(x *xorm.Engine) error {
	if err := dropIndexIfExists(x, "action", "IDX_action_c_u"); err != nil {
		return err
	}
	if err := dropIndexIfExists(x, "action", "IDX_action_r_u"); err != nil {
		return err
	}
	if err := dropIndexIfExists(x, "action", "IDX_action_user_id"); err != nil {
		return err
	}

	return x.Sync(new(v14bAction)) // nosemgrep:xorm-sync-missing-ignore-drop-indices
}
