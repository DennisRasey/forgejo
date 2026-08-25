// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"

	"code.forgejo.org/xorm/xorm"
	"code.forgejo.org/xorm/xorm/schemas"
)

// RecreateTables returns a function that will recreate the tables for the provided beans using the newly provided bean
// definition, move all data to the new tables, and then replace the original tables with a drop and rename.
//
// If any 'base' table is requested to be rebuilt where one-or-more 'satellite' tables exists that references it through
// a foreign key, you must rebuild the satellite tables as well or you will receive an error 'incomplete table set'.
//
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
func RecreateTables(beans ...any) func(*xorm.Engine) error {
	return func(x *xorm.Engine) error {
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			return err
		}
		sess = sess.StoreEngine("InnoDB")

		tableNames := make(map[any]string, len(beans))
		tempTableNames := make(map[any]string, len(beans))
		tempTableNamesByOriginalName := make(map[string]string, len(beans))
		for _, bean := range beans {
			tableName := sess.Engine().TableName(bean)
			tableNames[bean] = tableName
			tempTableName := fmt.Sprintf("tmp_recreate__%s", tableName)
			tempTableNames[bean] = tempTableName
			tempTableNamesByOriginalName[tableName] = tempTableName
		}

		// Create a set of temp tables.
		for _, bean := range beans {
			log.Info("Creating temp table: %s for Bean: %s", tempTableNames[bean], reflect.Indirect(reflect.ValueOf(bean)).Type().Name())
			if err := createTempTable(sess, bean, tempTableNames[bean]); err != nil {
				return err
			}
		}

		// Our new temp tables tables will have foreign keys that point to the original tables we are recreating.
		// Before we put data into these tables, we need to drop those foreign keys and add new foreign keys that point
		// to the temp tables.
		tableSchemas := make(map[any]*schemas.Table, len(beans))
		for _, bean := range beans {
			tableSchema, err := sess.Engine().TableInfo(bean)
			if err != nil {
				log.Error("Unable to get table info. Error: %v", err)
				return err
			}
			tableSchemas[bean] = tableSchema
			modifications := make([]schemas.TableModification, 0, len(tableSchema.ForeignKeys)*2)
			for _, fk := range tableSchema.ForeignKeys {
				targetTempTableName, ok := tempTableNamesByOriginalName[fk.TargetTableName]
				if !ok {
					return fmt.Errorf("incomplete table set: Found a foreign key reference to table %s, but it is not included in RecreateTables", fk.TargetTableName)
				}
				fkName := fk.Name
				if setting.Database.Type.IsMySQL() {
					// See MySQL explanation in createTempTable.
					fkName = "_" + fkName
				}
				modifications = append(modifications, schemas.DropForeignKey{ForeignKey: schemas.ForeignKey{
					Name:            fkName,
					SourceFieldName: fk.SourceFieldName,
					TargetTableName: fk.TargetTableName,
					TargetFieldName: fk.TargetFieldName,
				}})
				modifications = append(modifications, schemas.AddForeignKey{ForeignKey: schemas.ForeignKey{
					Name:            fkName,
					SourceFieldName: fk.SourceFieldName,
					TargetTableName: targetTempTableName, // FK changed to new temp table
					TargetFieldName: fk.TargetFieldName,
				}})
			}

			if len(modifications) != 0 {
				log.Info("Modifying temp table %s foreign keys to point to other temp tables", tempTableNames[bean])
				if err := sess.Table(tempTableNames[bean]).AlterTable(bean, modifications...); err != nil {
					return fmt.Errorf("alter table failed: while rewriting foreign keys to temp tables, error occurred: %w", err)
				}
			}
		}

		// Insert into the set of temp tables in the right order, starting with base tables, working outwards to
		// satellite tables.
		orderedBeans := slices.Clone(beans)
		slices.SortFunc(orderedBeans, func(b1, b2 any) int {
			return db.TableNameInsertionOrderSortFunc(tableNames[b1], tableNames[b2])
		})
		for _, bean := range orderedBeans {
			log.Info("Copying table %s to temp table %s", tableNames[bean], tempTableNames[bean])
			if err := copyData(sess, bean, tableNames[bean], tempTableNames[bean]); err != nil {
				// copyData does its own logging
				return err
			}
		}

		// Drop all the old tables in the right order, starting with satellite tables working inwards to base tables,
		// and rename all the temp tables to the final tables.  The database will automatically update the foreign key
		// references during the rename from temp to final tables.
		for i := len(orderedBeans) - 1; i >= 0; i-- {
			bean := orderedBeans[i]
			log.Info("Dropping existing table %s, and renaming temp table %s in its place", tableNames[bean], tempTableNames[bean])
			if err := renameTable(sess, bean, tableNames[bean], tempTableNames[bean], tableSchemas[bean]); err != nil {
				// renameTable does its own logging
				return err
			}
		}

		return sess.Commit()
	}
}

// LegacyRecreateTable will recreate the table using the newly provided bean definition and move all data to that new
// table.
//
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
//
// WARNING: YOU MUST COMMIT THE SESSION AT THE END
//
// Deprecated: LegacyRecreateTable exists for historical migrations and should not be used in current code -- tt does
// not support foreign key management.  Use RecreateTables instead which provides foreign key support.
func LegacyRecreateTable(sess *xorm.Session, bean any) error {
	tableName := sess.Engine().TableName(bean)
	tempTableName := fmt.Sprintf("tmp_recreate__%s", tableName)

	tableSchema, err := sess.Engine().TableInfo(bean)
	if err != nil {
		log.Error("Unable to get table info. Error: %v", err)
		return err
	}

	// We need to move the old table away and create a new one with the correct columns
	// We will need to do this in stages to prevent data loss
	//
	// First create the temporary table
	if err := createTempTable(sess, bean, tempTableName); err != nil {
		// createTempTable does its own logging
		return err
	}

	if err := copyData(sess, bean, tableName, tempTableName); err != nil {
		// copyData does its own logging
		return err
	}

	if err := renameTable(sess, bean, tableName, tempTableName, tableSchema); err != nil {
		// renameTable does its own logging
		return err
	}

	return nil
}

func createTempTable(sess *xorm.Session, bean any, tempTableName string) error {
	if setting.Database.Type.IsMySQL() {
		// Can't have identical foreign key names in MySQL, and Table(tempTableName) only affects the table name and not
		// the schema definition generated from the bean, so, we do a little adjusting by appending a `_` at the
		// beginning of each foreign key name on the temp table. We'll remove this by renaming the constraint after we
		// drop the original table, in renameTable.
		originalTableSchema, err := sess.Engine().TableInfo(bean)
		if err != nil {
			log.Error("Unable to get table info. Error: %v", err)
			return err
		}

		// `TableInfo()` will return a `*schema.Table` that is stored in a shared cache.  We don't want to mutate that
		// object as it will stick around and affect other things.  Make a mostly-shallow clone, with a new slice for
		// what we're changing.
		tableSchema := *originalTableSchema
		tableSchema.ForeignKeys = slices.Clone(originalTableSchema.ForeignKeys)
		for i := range tableSchema.ForeignKeys {
			tableSchema.ForeignKeys[i].Name = "_" + tableSchema.ForeignKeys[i].Name
		}

		sql, _, err := sess.Engine().Dialect().CreateTableSQL(&tableSchema, tempTableName)
		if err != nil {
			log.Error("Unable to generate CREATE TABLE query. Error: %v", err)
			return err
		}
		_, err = sess.Exec(sql)
		if err != nil {
			log.Error("Unable to create table %s. Error: %v", tempTableName, err)
			return err
		}
	} else {
		if err := sess.Table(tempTableName).CreateTable(bean); err != nil {
			log.Error("Unable to create table %s. Error: %v", tempTableName, err)
			return err
		}
	}

	if err := sess.Table(tempTableName).CreateUniques(bean); err != nil {
		log.Error("Unable to create uniques for table %s. Error: %v", tempTableName, err)
		return err
	}

	if err := sess.Table(tempTableName).CreateIndexes(bean); err != nil {
		log.Error("Unable to create indexes for table %s. Error: %v", tempTableName, err)
		return err
	}

	return nil
}

func copyData(sess *xorm.Session, bean any, tableName, tempTableName string) error {
	// Work out the column names from the bean - these are the columns to select from the old table and install into the new table
	table, err := sess.Engine().TableInfo(bean)
	if err != nil {
		log.Error("Unable to get table info. Error: %v", err)
		return err
	}
	newTableColumns := table.Columns()
	if len(newTableColumns) == 0 {
		return errors.New("no columns in new table")
	}
	hasID := false
	for _, column := range newTableColumns {
		hasID = hasID || (column.IsPrimaryKey && column.IsAutoIncrement)
	}

	sqlStringBuilder := &strings.Builder{}
	_, _ = sqlStringBuilder.WriteString("INSERT INTO `")
	_, _ = sqlStringBuilder.WriteString(tempTableName)
	_, _ = sqlStringBuilder.WriteString("` (`")
	_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
	_, _ = sqlStringBuilder.WriteString("`")
	for _, column := range newTableColumns[1:] {
		_, _ = sqlStringBuilder.WriteString(", `")
		_, _ = sqlStringBuilder.WriteString(column.Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}
	_, _ = sqlStringBuilder.WriteString(")")
	_, _ = sqlStringBuilder.WriteString(" SELECT ")
	if newTableColumns[0].Default != "" {
		_, _ = sqlStringBuilder.WriteString("COALESCE(`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`, ")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Default)
		_, _ = sqlStringBuilder.WriteString(")")
	} else {
		_, _ = sqlStringBuilder.WriteString("`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}

	for _, column := range newTableColumns[1:] {
		if column.Default != "" {
			_, _ = sqlStringBuilder.WriteString(", COALESCE(`")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`, ")
			_, _ = sqlStringBuilder.WriteString(column.Default)
			_, _ = sqlStringBuilder.WriteString(")")
		} else {
			_, _ = sqlStringBuilder.WriteString(", `")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`")
		}
	}
	_, _ = sqlStringBuilder.WriteString(" FROM `")
	_, _ = sqlStringBuilder.WriteString(tableName)
	_, _ = sqlStringBuilder.WriteString("`")

	if _, err := sess.Exec(sqlStringBuilder.String()); err != nil {
		log.Error("Unable to set copy data in to temp table %s. Error: %v", tempTableName, err)
		return err
	}

	return nil
}

func renameTable(sess *xorm.Session, bean any, tableName, tempTableName string, tableSchema *schemas.Table) error {
	switch {
	case setting.Database.Type.IsSQLite3():
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tempTableName).DropIndexes(bean); err != nil {
			log.Error("Unable to drop indexes on temporary table %s. Error: %v", tempTableName, err)
			return err
		}

		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateIndexes(bean); err != nil {
			log.Error("Unable to recreate indexes on table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateUniques(bean); err != nil {
			log.Error("Unable to recreate uniques on table %s. Error: %v", tableName, err)
			return err
		}

	case setting.Database.Type.IsMySQL():
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// MySQL will move all the constraints that reference this table from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		// In `RecreateTables` the foreign keys were renamed with a '_' prefix to avoid conflicting on the original
		// table's constraint names. Now that table has been dropped, so we can rename them back to leave the table in
		// the right state. Unfortunately this will cause a recheck of the constraint's validity against the target
		// table which will be slow for large tables, but it's unavoidable without the ability to rename constraints
		// in-place. Awkwardly these FKs are still a reference to the tmp_recreate target table since we drop in reverse
		// FK order -- the ALTER TABLE ... RENAME .. on those tmp tables will correct the FKs later.
		modifications := make([]schemas.TableModification, 0, len(tableSchema.ForeignKeys)*2)
		for _, fk := range tableSchema.ForeignKeys {
			modifications = append(modifications, schemas.DropForeignKey{ForeignKey: schemas.ForeignKey{
				Name:            "_" + fk.Name,
				SourceFieldName: fk.SourceFieldName,
				TargetTableName: fmt.Sprintf("tmp_recreate__%s", fk.TargetTableName),
				TargetFieldName: fk.TargetFieldName,
			}})
			modifications = append(modifications, schemas.AddForeignKey{ForeignKey: schemas.ForeignKey{
				Name:            fk.Name,
				SourceFieldName: fk.SourceFieldName,
				TargetTableName: fmt.Sprintf("tmp_recreate__%s", fk.TargetTableName),
				TargetFieldName: fk.TargetFieldName,
			}})
		}
		if len(modifications) != 0 {
			if err := sess.Table(tableName).AlterTable(bean, modifications...); err != nil {
				return fmt.Errorf("alter table failed: while rewriting foreign keys to original names, error occurred: %w", err)
			}
		}

	case setting.Database.Type.IsPostgreSQL():
		var originalSequences []string
		type sequenceData struct {
			LastValue int  `xorm:"'last_value'"`
			IsCalled  bool `xorm:"'is_called'"`
		}
		sequenceMap := map[string]sequenceData{}

		schema := sess.Engine().Dialect().URI().Schema
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_schema = ? AND (sequence_name LIKE ? || '_id_seq' AND sequence_catalog = ?)", schema, tableName, setting.Database.Name).Find(&originalSequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range originalSequences {
			sequenceData := sequenceData{}
			if _, err := sess.Table(sequence).Cols("last_value", "is_called").Get(&sequenceData); err != nil {
				log.Error("Unable to get last_value and is_called from %s. Error: %v", sequence, err)
				return err
			}
			sequenceMap[sequence] = sequenceData
		}

		// CASCADE causes postgres to drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s` CASCADE", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// CASCADE causes postgres to move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		var indices []string
		sess.Engine().SetSchema("")
		if err := sess.Table("pg_indexes").Cols("indexname").Where("tablename = ? AND schemaname = ?", tableName, schema).Find(&indices); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, index := range indices {
			newIndexName := strings.Replace(index, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER INDEX `%s` RENAME TO `%s`", index, newIndexName)); err != nil {
				log.Error("Unable to rename %s to %s. Error: %v", index, newIndexName, err)
				return err
			}
		}

		var sequences []string
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_schema = ? AND sequence_name LIKE 'tmp_recreate__' || ? || '_id_seq' AND sequence_catalog = ?", schema, tableName, setting.Database.Name).Find(&sequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range sequences {
			newSequenceName := strings.Replace(sequence, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER SEQUENCE `%s` RENAME TO `%s`", sequence, newSequenceName)); err != nil {
				log.Error("Unable to rename %s sequence to %s. Error: %v", sequence, newSequenceName, err)
				return err
			}
			val, ok := sequenceMap[newSequenceName]
			if newSequenceName == tableName+"_id_seq" {
				if ok && val.LastValue != 0 {
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
						log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
						return err
					}
				} else {
					// We're going to try to guess this
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', COALESCE((SELECT MAX(id)+1 FROM `%s`), 1), false)", newSequenceName, tableName)); err != nil {
						log.Error("Unable to reset %s. Error: %v", newSequenceName, err)
						return err
					}
				}
			} else if ok {
				if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
					log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
					return err
				}
			}
		}

	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}

// WARNING: YOU MUST COMMIT THE SESSION AT THE END
func DropTableColumns(sess *xorm.Session, tableName string, columnNames ...string) (err error) {
	if tableName == "" || len(columnNames) == 0 {
		return nil
	}
	// TODO: This will not work if there are foreign keys

	switch {
	case setting.Database.Type.IsSQLite3():
		// First drop the indexes on the columns
		res, errIndex := sess.Query(fmt.Sprintf("PRAGMA index_list(`%s`)", tableName))
		if errIndex != nil {
			return errIndex
		}
		for _, row := range res {
			indexName := row["name"]
			indexRes, err := sess.Query(fmt.Sprintf("PRAGMA index_info(`%s`)", indexName))
			if err != nil {
				return err
			}
			containsDroppedColumn := false
			for _, r := range indexRes {
				indexCol := string(r["name"])
				if slices.Contains(columnNames, indexCol) {
					containsDroppedColumn = true
					break
				}
			}
			if containsDroppedColumn {
				if _, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s`", indexName)); err != nil {
					return err
				}
			}
		}
		for _, col := range columnNames {
			if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", tableName, col)); err != nil {
				return fmt.Errorf("drop table `%s` column %s encountered error: %w", tableName, col, err)
			}
		}

	case setting.Database.Type.IsPostgreSQL():
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "` CASCADE"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.Database.Type.IsMySQL():
		// Drop indexes on columns first
		sql := fmt.Sprintf("SHOW INDEX FROM %s WHERE column_name IN ('%s')", tableName, strings.Join(columnNames, "','"))
		res, err := sess.Query(sql)
		if err != nil {
			return err
		}
		for _, index := range res {
			indexName := index["column_name"]
			if len(indexName) > 0 {
				_, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, tableName))
				if err != nil {
					return err
				}
			}
		}

		// Now drop the columns
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "`"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}

// ModifyColumn will modify column's type or other property. SQLITE is not supported
func ModifyColumn(x *xorm.Engine, tableName string, col *schemas.Column) error {
	var indexes map[string]*schemas.Index
	var err error

	defer func() {
		for _, index := range indexes {
			_, err = x.Exec(x.Dialect().CreateIndexSQL(tableName, index))
			if err != nil {
				log.Error("Create index %s on table %s failed: %v", index.Name, tableName, err)
			}
		}
	}()

	alterSQL := x.Dialect().ModifyColumnSQL(tableName, col)
	if _, err := x.Exec(alterSQL); err != nil {
		return err
	}
	return nil
}
