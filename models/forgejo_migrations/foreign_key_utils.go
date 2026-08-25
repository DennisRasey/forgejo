// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"fmt"

	"forgejo.org/modules/log"

	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

// syncForeignKeyWithDelete will delete any records that match `cond`, and if present, log and warn to the
// administrator; then it will perform an `xorm.Sync()` in order to create foreign keys on the table definition.
func syncForeignKeyWithDelete(x *xorm.Engine, bean any, cond builder.Cond) error {
	rowsDeleted, err := x.Where(cond).Delete(bean)
	if err != nil {
		return fmt.Errorf("failure to delete inconsistent records before foreign key sync: %w", err)
	}
	if rowsDeleted > 0 {
		tableName := x.TableName(bean)
		log.Warn(
			"Foreign key creation on table %s required deleting %d records with inconsistent foreign key values.",
			tableName, rowsDeleted)
	}

	// Sync() drops indexes by default, which will cause unnecessary rebuilding of indexes when syncForeignKeyWithDelete
	// is used with partial bean definitions; so we disable that option
	_, err = x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, bean)
	return err
}
