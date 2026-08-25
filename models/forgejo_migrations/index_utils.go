// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"fmt"
	"strings"

	"forgejo.org/modules/setting"

	"code.forgejo.org/xorm/xorm"
)

func dropIndexIfExists(x *xorm.Engine, tableName, indexName string) error {
	switch {
	case setting.Database.Type.IsSQLite3(), setting.Database.Type.IsPostgreSQL():
		if _, err := x.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", x.Quote(indexName))); err != nil {
			return err
		}

	case setting.Database.Type.IsMySQL():
		exists, err := indexExists(x, tableName, indexName)
		if err != nil {
			return err
		}

		if exists {
			if _, err := x.Exec(fmt.Sprintf("DROP INDEX %s ON %s", x.Quote(indexName), x.Quote(tableName))); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported db dialect type %v", x.Dialect().URI().DBType)
	}

	return nil
}

func indexExists(x *xorm.Engine, tableName, indexName string) (bool, error) {
	switch {
	case setting.Database.Type.IsSQLite3():
		return x.SQL("SELECT name FROM sqlite_master WHERE type = 'index' and name = ?", indexName).Exist()
	case setting.Database.Type.IsPostgreSQL():
		return x.SQL("SELECT indexname FROM pg_indexes WHERE schemaname = ? AND tablename = ? AND indexname = ?", setting.Database.Schema, tableName, indexName).Exist()
	case setting.Database.Type.IsMySQL():
		databaseName := strings.SplitN(setting.Database.Name, "?", 2)[0]
		return x.SQL("SELECT `INDEX_NAME` FROM `INFORMATION_SCHEMA`.`STATISTICS` WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ? AND `INDEX_NAME` = ?", databaseName, tableName, indexName).Exist()
	}

	return false, fmt.Errorf("unsupported db dialect type %v", x.Dialect().URI().DBType)
}
