// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/modules/optional"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add time zone support to action_schedule_spec",
		Upgrade:     addActionScheduleSpecTimeZone,
	})
}

func addActionScheduleSpecTimeZone(x *xorm.Engine) error {
	type ActionScheduleSpec struct {
		TimeZone optional.Option[string]
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionScheduleSpec))
	if err != nil {
		return err
	}

	_, err = x.Exec("ALTER TABLE action_schedule DROP COLUMN `specs`")
	return err
}
