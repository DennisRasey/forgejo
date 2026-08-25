// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "Add the column workflow_directory to the tables action_run and action_schedule",
		Upgrade:     addActionRunWorkflowDirectory,
	})
}

func addActionRunWorkflowDirectory(x *xorm.Engine) error {
	type ActionRun struct {
		WorkflowDirectory string `xorm:"workflow_directory NOT NULL DEFAULT '.forgejo/workflows'"`
	}
	type ActionSchedule struct {
		WorkflowDirectory string `xorm:"workflow_directory NOT NULL DEFAULT '.forgejo/workflows'"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRun), new(ActionSchedule))
	return err
}
