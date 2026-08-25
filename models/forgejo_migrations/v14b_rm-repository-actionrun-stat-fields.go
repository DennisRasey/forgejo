// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"fmt"

	"code.forgejo.org/xorm/xorm"
	"code.forgejo.org/xorm/xorm/schemas"
)

func init() {
	registerMigration(&Migration{
		Description: "remove from table repository fields num_action_runs, num_closed_action_runs",
		Upgrade:     removeRepositoryActionRunStatFields,
	})
}

func removeRepositoryActionRunStatFields(x *xorm.Engine) error {
	switch x.Dialect().URI().DBType {
	case schemas.SQLITE:
		// Can't drop multiple columns in one statement in SQLite.
		_, err := x.Exec("ALTER TABLE `repository` DROP COLUMN num_action_runs")
		if err != nil {
			return err
		}
		_, err = x.Exec("ALTER TABLE `repository` DROP COLUMN num_closed_action_runs")
		if err != nil {
			return err
		}
	case schemas.MYSQL, schemas.POSTGRES:
		_, err := x.Exec("ALTER TABLE `repository` DROP COLUMN num_action_runs, DROP COLUMN num_closed_action_runs")
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported db dialect type %v", x.Dialect().URI().DBType)
	}
	return nil
}
