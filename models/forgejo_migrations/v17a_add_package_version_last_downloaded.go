// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/models/db"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "Add LastDownloadUnix to PackageVersion",
		Upgrade:     addLastDownloadToPackageVersion,
	})
}

func addLastDownloadToPackageVersion(x *xorm.Engine) error {
	type PackageVersion struct {
		ID               int64              `xorm:"pk autoincr"`
		DownloadCount    int64              `xorm:"NOT NULL DEFAULT 0"`
		LastDownloadUnix timeutil.TimeStamp `xorm:"NULL"`
	}

	_, err := x.SyncWithOptions(
		xorm.SyncOptions{IgnoreDropIndices: true},
		new(PackageVersion),
	)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(db.DefaultContext).Exec("UPDATE `package_version` SET `last_download_unix` = ? WHERE download_count > 0", timeutil.TimeStampNow())
	return err
}
