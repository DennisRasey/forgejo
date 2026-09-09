// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "Add [Keep|Remove]LastDownloadDays to PackageCleanupRule",
		Upgrade:     addKeepRemoveLastDownloadDaysToPackageCleanupRule,
	})
}

func addKeepRemoveLastDownloadDaysToPackageCleanupRule(x *xorm.Engine) error {
	type PackageCleanupRule struct {
		KeepLastDownloadDays   int64 `xorm:"NOT NULL DEFAULT 0"`
		RemoveLastDownloadDays int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(
		xorm.SyncOptions{IgnoreDropIndices: true},
		new(PackageCleanupRule),
	)
	return err
}
