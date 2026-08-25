// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add forgejo_migration table",
		Upgrade:     addForgejoMigration,
	})
}

func addForgejoMigration(x *xorm.Engine) error {
	type ForgejoMigration struct {
		ID          string             `xorm:"pk"`
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
	}
	return x.Sync(new(ForgejoMigration)) // nosemgrep:xorm-sync-missing-ignore-drop-indices
}
