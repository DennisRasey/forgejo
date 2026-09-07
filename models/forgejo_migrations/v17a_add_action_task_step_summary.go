// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add action_task_step_summary table",
		Upgrade:     addActionTaskStepSummary,
	})
}

func addActionTaskStepSummary(x *xorm.Engine) error {
	type ActionTaskStepSummary struct {
		ID      int64  `xorm:"pk autoincr"`
		StepID  int64  `xorm:"UNIQUE NOT NULL REFERENCES(action_task_step, id)"`
		TaskID  int64  `xorm:"INDEX NOT NULL REFERENCES(action_task, id)"`
		RepoID  int64  `xorm:"INDEX NOT NULL REFERENCES(repository, id)"`
		Content string `xorm:"MEDIUMTEXT NOT NULL"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionTaskStepSummary))
	return err
}
