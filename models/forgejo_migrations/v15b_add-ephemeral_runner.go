// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add ephemeral to action_runner",
		Upgrade:     addRunnerEphemeralField,
	})
}

func addRunnerEphemeralField(x *xorm.Engine) error {
	type ActionRunner struct {
		Ephemeral bool `xorm:"ephemeral NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunner))
	return err
}
