// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddActionRunConcurrency(x *xorm.Engine) error {
	type ActionRun struct {
		RepoID           int64  `xorm:"index unique(repo_index) index(concurrency)"`
		Index            int64  `xorm:"index unique(repo_index)"`
		ConcurrencyGroup string `xorm:"index(concurrency)"`
		ConcurrencyType  int
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		// Sync drops indexes by default, and this local ActionRun doesn't have all the indexes -- so disable that.
		IgnoreDropIndices: true,
	}, new(ActionRun))
	return err
}
