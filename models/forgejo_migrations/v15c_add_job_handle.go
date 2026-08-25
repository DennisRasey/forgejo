// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add handle to action_run_job",
		Upgrade:     addActionRunJobHandle,
	})
}

func addActionRunJobHandle(x *xorm.Engine) error {
	type ActionRunJob struct {
		Handle string `xorm:"unique"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob))
	return err
}
