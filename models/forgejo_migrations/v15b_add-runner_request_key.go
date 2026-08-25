// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add runner_request_key to action_task",
		Upgrade:     addActionTaskRunnerRequestKey,
	})
}

func addActionTaskRunnerRequestKey(x *xorm.Engine) error {
	type ActionTask struct {
		RunnerID         int64  `xorm:"index index(request_key)"`
		RunnerRequestKey string `xorm:"index(request_key)"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionTask))
	return err
}
