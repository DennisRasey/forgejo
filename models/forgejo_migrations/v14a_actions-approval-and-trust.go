// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"context"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add actions approval and trust table and fields",
		Upgrade:     v14ActionsApprovalAndTrust,
	})
}

func v14ActionsApprovalAndTrust(x *xorm.Engine) error {
	if err := v14ActionsApprovalAndTrustCreateTableActionUser(x); err != nil {
		return err
	}
	if err := v14ActionsApprovalAndTrustAddActionsRunFields(x); err != nil {
		return err
	}
	return v14ActionsApprovalAndTrustPopulateTableActionUser(x)
}

func v14ActionsApprovalAndTrustCreateTableActionUser(x *xorm.Engine) error {
	type ActionUser struct {
		ID     int64 `xorm:"pk autoincr"`
		UserID int64 `xorm:"INDEX UNIQUE(action_user_index) REFERENCES(user, id)"`
		RepoID int64 `xorm:"INDEX UNIQUE(action_user_index) REFERENCES(repository, id)"`

		TrustedWithPullRequests bool

		LastAccess timeutil.TimeStamp `xorm:"INDEX"`
	}
	return x.Sync(new(ActionUser)) // nosemgrep:xorm-sync-missing-ignore-drop-indices
}

func v14ActionsApprovalAndTrustAddActionsRunFields(x *xorm.Engine) error {
	type ActionRun struct {
		PullRequestPosterID int64
		PullRequestID       int64 `xorm:"index"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRun))
	return err
}

type v14ActionsApprovalAndTrustTrusted struct {
	RepoID int64
	UserID int64
}

func v14ActionsApprovalAndTrustPopulateTableActionUser(x *xorm.Engine) error {
	type ActionUser struct {
		ID                      int64 `xorm:"pk autoincr"`
		UserID                  int64 `xorm:"INDEX UNIQUE(action_user_index) REFERENCES(user, id)"`
		RepoID                  int64 `xorm:"INDEX UNIQUE(action_user_index) REFERENCES(repository, id)"`
		TrustedWithPullRequests bool
		LastAccess              timeutil.TimeStamp `xorm:"INDEX"`
	}
	insertActionUser := func(ctx context.Context, user *ActionUser) error {
		user.LastAccess = timeutil.TimeStampNow()
		return db.Insert(ctx, user)
	}

	//
	// Users approved once were trusted before and are trusted now.
	//
	// The admin will see they can revoke that trust when the user
	// submits a new pull request.
	//
	// If the user does not submit any pull request, this trust will
	// eventually be automatically revoked.
	//
	// The number of trusted users is assumed to be small enough to not require
	// pagination, even on large instances.
	//
	log.Info("v14a_actions-approval-and-trust: search")
	var trustedList []*v14ActionsApprovalAndTrustTrusted
	if err := x.Table("`action_run`").
		Select("DISTINCT `action_run`.`repo_id`, `action_run`.`trigger_user_id` AS `user_id`").
		Join("INNER", "`repository`", "`repository`.`id` = `action_run`.`repo_id`").
		Join("INNER", "`user`", "`user`.`id` = `action_run`.`trigger_user_id`").
		Where("`action_run`.`approved_by` > 0 AND `action_run`.`trigger_user_id` > 0").
		OrderBy("`action_run`.`repo_id`, `action_run`.`trigger_user_id`").
		Find(&trustedList); err != nil {
		return err
	}

	log.Info("v14a_actions-approval-and-trust: start adding %d users trusted with workflow runs", len(trustedList))
	if err := db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		for _, trusted := range trustedList {
			log.Debug("v14a_actions-approval-and-trust: repository %d trusts user %d", trusted.RepoID, trusted.UserID)
			if err := insertActionUser(ctx, &ActionUser{
				RepoID:                  trusted.RepoID,
				UserID:                  trusted.UserID,
				TrustedWithPullRequests: true,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	log.Info("v14a_actions-approval-and-trust: done adding %d users", len(trustedList))
	return nil
}
