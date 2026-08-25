// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/models/gitea_migrations/base"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "remove is_deleted column from activity action table",
		Upgrade:     removeIsDeletedColumnFromActivityActionTable,
	})
}

func removeIsDeletedColumnFromActivityActionTable(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Table("action").Where("is_deleted = ?", true).Delete(); err != nil {
		return err
	}

	if err := base.DropTableColumns(sess, "action", "is_deleted"); err != nil {
		return err
	}
	return sess.Commit()
}
