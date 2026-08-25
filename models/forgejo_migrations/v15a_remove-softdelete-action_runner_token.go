// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

func init() {
	registerMigration(&Migration{
		Description: "remove soft-delete capability from action_runner_token",
		Upgrade:     removeSoftDeleteActionRunnerToken,
	})
}

func removeSoftDeleteActionRunnerToken(x *xorm.Engine) error {
	// ActionRunnerToken was implemented with a column: "Deleted timeutil.TimeStamp `xorm:"deleted"``", which invokes
	// xorm's soft-delete capability -- that is, if a record is deleted from the table, then it is just marked with a
	// delete timestamp which causes it to be automatically excluded from future queries.  This functionality is not
	// used on `ActionRunnerToken` and it stands in the way of foreign key implementation -- if you can't actually
	// delete the record in the table, then you can't remove foreign key references and therefore can't delete contents
	// of the target tables, repository and user.
	//
	// This migration removes that column and deletes any records that were soft-deleted.

	// Before dropping the 'deleted' column, hard-delete any soft-deleted records.
	if _, err := x.Table("action_runner_token").Where(builder.NotNull{"deleted"}).Delete(); err != nil {
		return err
	}

	_, err := x.Exec("ALTER TABLE action_runner_token DROP COLUMN `deleted`")
	return err
}
