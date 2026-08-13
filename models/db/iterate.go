// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"forgejo.org/modules/setting"

	"xorm.io/builder"
)

// Iterate iterate all the Bean object. The table being iterated must have a single-column primary key.
func Iterate[Bean any](ctx context.Context, cond builder.Cond, f func(ctx context.Context, bean *Bean) error) error {
	var dummy Bean
	batchSize := setting.Database.IterateBufferSize

	table, err := TableInfo(&dummy)
	if err != nil {
		return fmt.Errorf("unable to fetch table info for bean %v: %w", dummy, err)
	}
	if len(table.PrimaryKeys) != 1 {
		return fmt.Errorf("iterate only supported on a table with 1 primary key field, but table %s had %d", table.Name, len(table.PrimaryKeys))
	}

	pkDbName := table.PrimaryKeys[0]
	var pkStructFieldName string

	for _, c := range table.Columns() {
		if c.Name == pkDbName {
			pkStructFieldName = c.FieldName
			break
		}
	}
	if pkStructFieldName == "" {
		return fmt.Errorf("iterate unable to identify struct field for primary key %s", pkDbName)
	}

	var lastPK any

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			beans := make([]*Bean, 0, batchSize)

			sess := GetEngine(ctx)
			sess = sess.OrderBy(pkDbName)
			if cond != nil {
				sess = sess.Where(cond)
			}
			if lastPK != nil {
				sess = sess.Where(builder.Gt{pkDbName: lastPK})
			}

			if err := sess.Limit(batchSize).Find(&beans); err != nil {
				return err
			}
			if len(beans) == 0 {
				return nil
			}

			for _, bean := range beans {
				if err := f(ctx, bean); err != nil {
					return err
				}
			}

			lastBean := beans[len(beans)-1]
			lastPK = extractFieldValue(lastBean, pkStructFieldName)
		}
	}
}

func extractFieldValue(bean any, fieldName string) any {
	v := reflect.ValueOf(bean)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	field := v.FieldByName(fieldName)
	return field.Interface()
}

// IterateByKeyset iterates all the records on a database (matching the provided condition) in the order of specified
// order fields, and invokes the provided handler function for each record. It is safe to UPDATE or DELETE the record in
// the handler function, as long as the order fields are not mutated on the record (which could cause records to be
// missed or iterated multiple times).
//
// Assuming order fields a, b, and c, then database queries will be performed as "SELECT * FROM table WHERE (a, b, c) >
// (last_a, last_b, last_c) ORDER BY a, b, c LIMIT buffer_size" repeatedly until the query returns no records (except
// the first query will have no WHERE clause).
//
// Critical requirements for proper usage:
//
// - the order fields encompass at least one UNIQUE or PRIMARY KEY constraint of the table to ensure that records are
// not duplicated -- for example, if the table has a unique index covering `(repo_id, index)`, then it would be safe to
// use this function as long as both fields (in either order) are provided as order fields.
//
// - none of the order fields may have NULL values in them, as the `=` and `>` comparisons being performed by the
// iterative queries will not operate on these records consistently as they do with other values.
//
// This implementation could be a much simpler streaming scan of the query results, except that doesn't permit making
// any additional database queries or data modifications in the target function -- SQLite cannot write while holding a
// read lock. Buffering pages of data in-memory avoids that issue.
//
// Performance:
//
// - High performance will result from an alignment of an index on the table with the order fields, in the same field
// order, even if additional ordering fields could be provided after the index fields. In the absence of this index
// alignment, it is reasonable to expect that every extra page of data accessed will require a query that will perform
// an index scan (if available) or sequential scan of the target table. In testing on the `commit_status` table with
// 455k records, a fully index-supported ordering allowed each query page to execute in 0.18ms, as opposed to 80ms
// per-query without matching supporting index.
//
// - In the absence of a matching index, slower per-query performance can be compensated with a larger `batchSize`
// parameter, which controls how many records to fetch at once and therefore reduces the number of queries required.
// This requires more memory. Similar `commit_status` table testing showed these stats for iteration time and memory
// usage for different buffer sizes; specifics will vary depending on the target table:
//   - buffer size = 1,000,000 - iterates in 2.8 seconds, consumes 363 MB of RAM
//   - buffer size = 100,000 - iterates in 3.5 seconds, consume 130 MB of RAM
//   - buffer size = 10,000 - iterates in 7.1 seconds, consumes 59 MB of RAM
//   - buffer size = 1,000 - iterates in 33.9 seconds, consumes 42 MB of RAM
func IterateByKeyset[Bean any](ctx context.Context, cond builder.Cond, orderFields []string, batchSize int, f func(ctx context.Context, bean *Bean) error) error {
	var dummy Bean

	if len(orderFields) == 0 {
		return errors.New("orderFields must be provided")
	}

	table, err := TableInfo(&dummy)
	if err != nil {
		return fmt.Errorf("unable to fetch table info for bean %v: %w", dummy, err)
	}
	goFieldNames := make([]string, len(orderFields))
	for i, f := range orderFields {
		goFieldNames[i] = table.GetColumn(f).FieldName
	}
	sqlFieldNames := make([]string, len(orderFields))
	for i, f := range orderFields {
		// Support field names like "index" which need quoting in builder.Cond & OrderBy
		sqlFieldNames[i] = x.Dialect().Quoter().Quote(f)
	}

	var lastKey []any

	// For the order fields, generate clauses (a, b, c) and (?, ?, ?) which will be used in the WHERE clause when
	// reading additional pages of data.
	rowValue := strings.Builder{}
	rowParameterValue := strings.Builder{}
	rowValue.WriteString("(")
	rowParameterValue.WriteString("(")
	for i, f := range sqlFieldNames {
		rowValue.WriteString(f)
		rowParameterValue.WriteString("?")
		if i != len(sqlFieldNames)-1 {
			rowValue.WriteString(", ")
			rowParameterValue.WriteString(", ")
		}
	}
	rowValue.WriteString(")")
	rowParameterValue.WriteString(")")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			beans := make([]*Bean, 0, batchSize)

			sess := GetEngine(ctx)
			for _, f := range sqlFieldNames {
				sess = sess.OrderBy(f)
			}
			if cond != nil {
				sess = sess.Where(cond)
			}
			if lastKey != nil {
				sess = sess.Where(
					builder.Expr(fmt.Sprintf("%s > %s", rowValue.String(), rowParameterValue.String()), lastKey...),
				)
			}

			if err := sess.Limit(batchSize).Find(&beans); err != nil {
				return err
			}
			if len(beans) == 0 {
				return nil
			}

			for _, bean := range beans {
				if err := f(ctx, bean); err != nil {
					return err
				}
			}

			lastBean := beans[len(beans)-1]
			lastKey = make([]any, len(goFieldNames))
			for i := range goFieldNames {
				lastKey[i] = extractFieldValue(lastBean, goFieldNames[i])
			}
		}
	}
}
