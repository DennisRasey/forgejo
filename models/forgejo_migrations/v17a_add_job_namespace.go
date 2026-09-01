// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add job_namespace to action_run_job",
		Upgrade:     addActionJobNamespace,
	})
}

func addActionJobNamespace(x *xorm.Engine) error {
	type JobNamespace string
	type ActionRunJob struct {
		JobNamespace JobNamespace `xorm:"VARCHAR(255) NULL"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob))
	return err
}
