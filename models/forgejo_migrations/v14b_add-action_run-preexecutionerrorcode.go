// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add PreExecutionErrorCode & PreExecutionErrorDetails to action_run",
		Upgrade:     addActionRunPreExecutionErrorCode,
	})
}

func addActionRunPreExecutionErrorCode(x *xorm.Engine) error {
	type ActionRun struct {
		PreExecutionErrorCode    int64
		PreExecutionErrorDetails []any `xorm:"JSON LONGTEXT"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRun))
	return err
}
