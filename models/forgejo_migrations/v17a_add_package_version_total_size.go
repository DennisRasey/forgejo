// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"context"

	"forgejo.org/models/db"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "Add TotalSize to PackageVersion",
		Upgrade:     addPackageVersionTotalSize,
	})
}

func addPackageVersionTotalSize(x *xorm.Engine) error {
	type PackageVersion struct {
		ID        int64 `xorm:"pk autoincr"`
		TotalSize int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	type PackageBlob struct {
		ID   int64 `xorm:"pk autoincr"`
		Size int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(
		xorm.SyncOptions{IgnoreDropIndices: true},
		new(PackageVersion),
	)
	if err != nil {
		return err
	}

	return db.Iterate(db.DefaultContext, nil, func(ctx context.Context, pv *PackageVersion) error {
		e := db.GetEngine(ctx)
		totalSize, err := e.
			Table("package_file").
			Where("package_file.version_id = ?", pv.ID).
			Cols("blob_id").
			Join("INNER", "package_blob", "package_file.blob_id = package_blob.id").
			SumInt(&PackageBlob{}, "package_blob.size")
		if err != nil {
			return err
		}

		pv.TotalSize = totalSize
		_, err = db.GetEngine(ctx).ID(pv.ID).Cols("total_size").Update(pv)

		return err
	})
}
