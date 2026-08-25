// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddActionRunPreExecutionError(x *xorm.Engine) error {
	type ActionRun struct {
		PreExecutionError string `xorm:"LONGTEXT"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		// Sync drops indexes by default, and this local ActionRun doesn't have all the indexes -- so disable that.
		IgnoreDropIndices: true,
	}, new(ActionRun))
	return err
}
