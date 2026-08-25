// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"cmp"
	"fmt"
	"slices"
	"sync"

	"forgejo.org/modules/container"

	"code.forgejo.org/xorm/xorm/schemas"
)

type schemaWithDefaultBean struct {
	schema *schemas.Table
	bean   any
}

var (
	cachedForeignKeyOrderedTables = sync.OnceValues(foreignKeyOrderedTables)
	cachedTableNameLookupOrder    = sync.OnceValues(tableNameLookupOrder)
	// Slice of all registered tables, including their bean from `RegisterModel()` and their schemas.Table reference.
	cachedSchemaTables = sync.OnceValues(func() ([]schemaWithDefaultBean, error) {
		schemaTables := make([]schemaWithDefaultBean, 0, len(tables))
		for _, bean := range tables {
			table, err := TableInfo(bean)
			if err != nil {
				return nil, fmt.Errorf("cachedSchemaTables: failure to fetch schema table for bean %#v: %w", bean, err)
			}
			schemaTables = append(schemaTables, schemaWithDefaultBean{
				schema: table,
				bean:   bean,
			})
		}
		return schemaTables, nil
	})
	// Lookup map from table name -> {schemas.Table, bean}. The bean is the empty bean from `RegisterModel()`.
	cachedTableMap = sync.OnceValues(func() (map[string]schemaWithDefaultBean, error) {
		schemaTables, err := cachedSchemaTables()
		if err != nil {
			return nil, err
		}
		retval := make(map[string]schemaWithDefaultBean, len(schemaTables))
		for _, table := range schemaTables {
			retval[table.schema.Name] = table
		}
		return retval, nil
	})
	// Table A has foreign keys to [B, C], this is a map of A -> {B, C}.
	cachedReferencingTables = sync.OnceValues(func() (map[string][]string, error) {
		schemaTables, err := cachedSchemaTables()
		if err != nil {
			return nil, err
		}
		return calculateReferencingTables(schemaTables), nil
	})
	// Table A has foreign keys to [B, C], this is a map of B -> {A}, C -> {A}.
	cachedReferencedTables = sync.OnceValues(func() (map[string][]string, error) {
		referencingTables, err := cachedReferencingTables()
		if err != nil {
			return nil, err
		}
		referencedTables := make(map[string][]string)
		for referencingTable, targetTables := range referencingTables {
			for _, targetTable := range targetTables {
				referencedTables[targetTable] = append(referencedTables[targetTable], referencingTable)
			}
		}
		return referencedTables, nil
	})
)

// Create a map for each schema table which contains a slice of all the tables that reference it (with a foreign key).
func calculateReferencingTables(tables []schemaWithDefaultBean) map[string][]string {
	referencingTables := make(map[string][]string, len(tables))
	for _, table := range tables {
		tableName := table.schema.Name
		for _, fk := range table.schema.ForeignKeys {
			referencingTables[tableName] = append(referencingTables[tableName], fk.TargetTableName)
		}
	}
	return referencingTables
}

// Create a list of database tables in their "foreign key order".  This order specifies the safe insertion order for
// records into tables, where earlier tables in the list are referenced by foreign keys that exist in tables later in
// the list.  This order can be used in reverse as a safe deletion order as well.
//
// An ordered list of tables is incompatible with tables that have self-referencing foreign keys and circular referenced
// foreign keys; however neither of those cases are in-use in Forgejo.
func calculateTableForeignKeyOrder(tables []schemaWithDefaultBean) ([]schemaWithDefaultBean, error) {
	remainingTables := slices.Clone(tables)

	referencingTables := calculateReferencingTables(remainingTables)
	orderedTables := make([]schemaWithDefaultBean, 0, len(remainingTables))

	for len(remainingTables) > 0 {
		nextGroup := make([]schemaWithDefaultBean, 0, len(remainingTables))

		for _, targetTable := range remainingTables {
			// Skip if this targetTable has foreign keys and the target table hasn't been created.
			slice, ok := referencingTables[targetTable.schema.Name]
			if ok && len(slice) > 0 { // This table is still referencing an uncreated table
				continue
			}
			// This table's references are satisfied or it had none
			nextGroup = append(nextGroup, targetTable)
		}

		if len(nextGroup) == 0 {
			return nil, fmt.Errorf("calculateTableForeignKeyOrder: unable to figure out next table from remainingTables = %#v", remainingTables)
		}

		orderedTables = append(orderedTables, nextGroup...)

		// Cleanup between loops: remove each table in nextGroup from remainingTables, and remove their table names from
		// referencingTables as well.
		for _, doneTable := range nextGroup {
			remainingTables = slices.DeleteFunc(remainingTables, func(remainingTable schemaWithDefaultBean) bool {
				return remainingTable.schema.Name == doneTable.schema.Name
			})
			for referencingTable, referencedTables := range referencingTables {
				referencingTables[referencingTable] = slices.DeleteFunc(referencedTables, func(tableName string) bool {
					return tableName == doneTable.schema.Name
				})
			}
		}
	}

	return orderedTables, nil
}

// Create a list of registered database tables in their "foreign key order", per calculateTableForeignKeyOrder.
func foreignKeyOrderedTables() ([]schemaWithDefaultBean, error) {
	schemaTables, err := cachedSchemaTables()
	if err != nil {
		return nil, err
	}

	orderedTables, err := calculateTableForeignKeyOrder(schemaTables)
	if err != nil {
		return nil, err
	}

	return orderedTables, nil
}

// Create a map from each registered database table's name to its order in "foreign key order", per
// calculateTableForeignKeyOrder.
func tableNameLookupOrder() (map[string]int, error) {
	tables, err := cachedForeignKeyOrderedTables()
	if err != nil {
		return nil, err
	}

	lookupMap := make(map[string]int, len(tables))
	for i, table := range tables {
		lookupMap[table.schema.Name] = i
	}

	return lookupMap, nil
}

// When used as a comparator function in `slices.SortFunc`, can sort a slice into the safe insertion order for records
// in tables, where earlier tables in the list are referenced by foreign keys that exist in tables later in the list.
func TableNameInsertionOrderSortFunc(table1, table2 string) int {
	lookupMap, err := cachedTableNameLookupOrder()
	if err != nil {
		panic(fmt.Sprintf("cachedTableNameLookupOrder failed: %#v", err))
	}

	// Since this is typically used by `slices.SortFunc` it can't return an error.  If a table is referenced that isn't
	// a registered model then it will be sorted at the beginning -- this case is used in models/gitea_migrations/test.
	val1, ok := lookupMap[table1]
	if !ok {
		val1 = -1
	}
	val2, ok := lookupMap[table2]
	if !ok {
		val2 = -1
	}

	return cmp.Compare(val1, val2)
}

// In "Insert" order, tables that have a foreign key will be sorted after the tables that the foreign key points to, so
// that records can be safely inserted in this order.  "Delete" order is the opposite, and allows records to be safely
// deleted in this order.
type foreignKeySortOrder int8

const (
	foreignKeySortInsert foreignKeySortOrder = iota
	foreignKeySortDelete
)

// Sort the provided beans in the provided foreign-key sort order.
func sortBeans(beans []any, sortOrder foreignKeySortOrder) ([]any, error) {
	type beanWithTableName struct {
		bean      any
		tableName string
	}

	beansWithTableNames := make([]beanWithTableName, 0, len(beans))
	for _, bean := range beans {
		table, err := TableInfo(bean)
		if err != nil {
			return nil, fmt.Errorf("sortBeans: failure to fetch schema table for bean %#v: %w", bean, err)
		}
		beansWithTableNames = append(beansWithTableNames, beanWithTableName{bean: bean, tableName: table.Name})
	}

	slices.SortFunc(beansWithTableNames, func(a, b beanWithTableName) int {
		if sortOrder == foreignKeySortInsert {
			return TableNameInsertionOrderSortFunc(a.tableName, b.tableName)
		}
		return TableNameInsertionOrderSortFunc(b.tableName, a.tableName)
	})

	beanRetval := make([]any, len(beans))
	for i, beanWithTableName := range beansWithTableNames {
		beanRetval[i] = beanWithTableName.bean
	}
	return beanRetval, nil
}

// A database operation on `beans` may need to affect additional tables based upon foreign keys to those beans.
// extendBeansForCascade returns a new list of beans which includes all the referencing tables that link to `beans`'
// tables. For example, provided a `&User{}`, it will return `[&User{}, &Stopwatch{}, ...]` where `Stopwatch` is a table
// that references `User`. The additional beans returned will be default structs that were provided to
// `db.RegisterModel`.
func extendBeansForCascade(beans []any) ([]any, error) {
	referencedTables, err := cachedReferencedTables()
	if err != nil {
		return nil, err
	}
	tableMap, err := cachedTableMap()
	if err != nil {
		return nil, err
	}

	deduplicateTables := make(container.Set[string], len(beans))

	finalBeans := slices.Clone(beans)
	newBeans := beans
	nextBeanSet := make([]any, 0)

	for {
		for _, bean := range newBeans {
			schema, err := TableInfo(bean)
			if err != nil {
				return nil, fmt.Errorf("cascadeDeleteTables: failure to fetch schema table for bean %#v: %w", bean, err)
			}
			if deduplicateTables.Contains(schema.Name) {
				continue
			}

			for _, column := range schema.Columns() {
				if column.IsDeleted {
					return nil, fmt.Errorf("unable to use table %q in a cascade operation, as it has a soft-delete column %q", schema.Name, column.FieldName)
				}
			}

			deduplicateTables.Add(schema.Name)
			for _, referencingTable := range referencedTables[schema.Name] {
				table := tableMap[referencingTable]
				finalBeans = append(finalBeans, table.bean)
				nextBeanSet = append(nextBeanSet, table.bean)
			}
		}

		if len(nextBeanSet) == 0 {
			break
		}
		newBeans = nextBeanSet
		nextBeanSet = nextBeanSet[:0] // set len 0, keep allocation for reuse
	}

	return finalBeans, nil
}
