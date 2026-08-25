// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/modules/setting"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "remove columns and rework indexes for notification table",
		Upgrade:     reworkNotification,
	})
}

func reworkNotification(x *xorm.Engine) error {
	type NotificationStatus uint8
	type Notification struct {
		UserID int64              `xorm:"NOT NULL INDEX(s)"`
		Status NotificationStatus `xorm:"SMALLINT NOT NULL INDEX(s)"`
	}

	if err := dropIndexIfExists(x, "notification", "IDX_notification_user_id"); err != nil {
		return err
	}

	if err := dropIndexIfExists(x, "notification", "IDX_notification_created_unix"); err != nil {
		return err
	}

	if err := dropIndexIfExists(x, "notification", "IDX_notification_updated_by"); err != nil {
		return err
	}

	if err := dropIndexIfExists(x, "notification", "IDX_notification_commit_id"); err != nil {
		return err
	}

	if err := dropIndexIfExists(x, "notification", "IDX_notification_status"); err != nil {
		return err
	}

	switch {
	case setting.Database.Type.IsSQLite3():

		if _, err := x.Exec("ALTER TABLE `notification` DROP COLUMN `updated_by`"); err != nil {
			return err
		}

		if _, err := x.Exec("ALTER TABLE `notification` DROP COLUMN `commit_id`"); err != nil {
			return err
		}

	case setting.Database.Type.IsMySQL(), setting.Database.Type.IsPostgreSQL():
		if _, err := x.Exec("ALTER TABLE `notification` DROP COLUMN `updated_by`, DROP COLUMN `commit_id`"); err != nil {
			return err
		}
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(Notification))
	return err
}
