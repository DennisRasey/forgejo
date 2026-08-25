// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import (
	"fmt"

	"forgejo.org/modules/log"

	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

// syncForeignKeyWithDelete will delete any records that match `cond`, and if present, log and warn to the
// administrator; then it will perform an `xorm.Sync()` in order to create foreign keys on the table definition.
func syncForeignKeyWithDelete(x *xorm.Engine, bean any, cond builder.Cond) error {
	rowsDeleted, err := x.Where(cond).Delete(bean)
	if err != nil {
		return fmt.Errorf("failure to delete inconsistent records before foreign key sync: %w", err)
	}
	if rowsDeleted > 0 {
		tableName := x.TableName(bean)
		log.Warn(
			"Foreign key creation on table %s required deleting %d records with inconsistent foreign key values.",
			tableName, rowsDeleted)
	}

	// Sync() drops indexes by default, which will cause unnecessary rebuilding of indexes when syncForeignKeyWithDelete
	// is used with partial bean definitions; so we disable that option
	_, err = x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, bean)
	return err
}

func AddForeignKeysStopwatchTrackedTime(x *xorm.Engine) error {
	type Stopwatch struct {
		IssueID int64 `xorm:"INDEX REFERENCES(issue, id)"`
		UserID  int64 `xorm:"INDEX REFERENCES(user, id)"`
	}
	type TrackedTime struct {
		ID      int64 `xorm:"pk autoincr"`
		IssueID int64 `xorm:"INDEX REFERENCES(issue, id)"`
		UserID  int64 `xorm:"INDEX REFERENCES(user, id)"`
	}

	// TrackedTime.UserID used to be an intentionally dangling reference if a user was deleted, in order to maintain the
	// time that was tracked against an issue.  With the addition of a foreign key, we set UserID to NULL where the user
	// doesn't exist instead of leaving it pointing to an invalid record:
	var trackedTime []TrackedTime
	err := x.Table("tracked_time").
		Join("LEFT", "`user`", "`tracked_time`.user_id = `user`.id").
		Where(builder.IsNull{"`user`.id"}).
		Where(builder.NotNull{"tracked_time.user_id"}).
		Find(&trackedTime)
	if err != nil {
		return err
	}
	for _, tt := range trackedTime {
		affected, err := x.Table(&TrackedTime{}).Where("id = ?", tt.ID).Update(map[string]any{"user_id": nil})
		if err != nil {
			return err
		} else if affected != 1 {
			return fmt.Errorf("expected to update 1 tracked_time record with ID %d, but actually affected %d records", tt.ID, affected)
		}
	}

	err = syncForeignKeyWithDelete(x,
		new(Stopwatch),
		builder.Or(
			builder.Expr("NOT EXISTS (SELECT id FROM issue WHERE issue.id = stopwatch.issue_id)"),
			builder.Expr("NOT EXISTS (SELECT id FROM `user` WHERE `user`.id = stopwatch.user_id)"),
		),
	)
	if err != nil {
		return err
	}

	return syncForeignKeyWithDelete(x,
		new(TrackedTime),
		builder.Or(
			builder.And(
				builder.Expr("user_id IS NOT NULL"),
				builder.Expr("NOT EXISTS (SELECT id FROM `user` WHERE `user`.id = tracked_time.user_id)"),
			),
			builder.Expr("NOT EXISTS (SELECT id FROM issue WHERE issue.id = tracked_time.issue_id)"),
		),
	)
}
