// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	"forgejo.org/models/db"
	"forgejo.org/models/gitea_migrations"
	"forgejo.org/modules/log"

	"code.forgejo.org/xorm/xorm"
)

func checkDBVersion(ctx context.Context, logger log.Logger, autofix bool) error {
	logger.Info("Expected database version: %d", gitea_migrations.ExpectedDBVersion())
	if err := db.InitEngineWithMigration(ctx, func(eng db.Engine) error {
		return gitea_migrations.EnsureUpToDate(eng.(*xorm.Engine))
	}); err != nil {
		if !autofix {
			logger.Critical("Error: %v during ensure up to date", err)
			return err
		}
		logger.Warn("Got Error: %v during ensure up to date", err)
		logger.Warn("Attempting to migrate to the latest DB version to fix this.")

		err = db.InitEngineWithMigration(ctx, func(eng db.Engine) error {
			return gitea_migrations.Migrate(eng.(*xorm.Engine))
		})
		if err != nil {
			logger.Critical("Error: %v during migration", err)
		}
		return err
	}
	return nil
}

func init() {
	Register(&Check{
		Title:         "Check Database Version",
		Name:          "check-db-version",
		IsDefault:     true,
		Run:           checkDBVersion,
		AbortIfFailed: false,
		Priority:      2,
	})
}
