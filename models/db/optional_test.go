// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package db_test

import (
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/optional"

	"code.forgejo.org/xorm/xorm/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionFieldInt(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	type OptionInt struct {
		ID     int64 `xorm:"pk autoincr"`
		Number optional.Option[int64]
	}
	db.GetEngine(t.Context()).Sync(&OptionInt{})

	t.Run("insert null, read back", func(t *testing.T) {
		null := OptionInt{
			Number: optional.None[int64](),
		}
		cnt, err := db.GetEngine(t.Context()).Insert(&null)
		require.NoError(t, err)
		assert.EqualValues(t, 1, cnt)

		{
			var read OptionInt
			has, err := db.GetEngine(t.Context()).ID(null.ID).Get(&read)
			require.NoError(t, err)
			require.True(t, has)
			hasNumber, _ := read.Number.Get()
			assert.False(t, hasNumber)
		}
	})

	t.Run("insert not null, read back", func(t *testing.T) {
		notNull := OptionInt{
			Number: optional.Some[int64](123),
		}

		cnt, err := db.GetEngine(t.Context()).Insert(&notNull)
		require.NoError(t, err)
		assert.EqualValues(t, 1, cnt)
		{
			var read OptionInt
			has, err := db.GetEngine(t.Context()).ID(notNull.ID).Get(&read)
			require.NoError(t, err)
			require.True(t, has)
			hasNumber, number := read.Number.Get()
			assert.True(t, hasNumber)
			assert.EqualValues(t, 123, number)
		}
	})

	t.Run("read multiple records without filters", func(t *testing.T) {
		var arr []OptionInt
		err := db.GetEngine(t.Context()).Find(&arr)
		require.NoError(t, err)
		assert.Len(t, arr, 2)
	})

	t.Run("read multiple records with bean filters", func(t *testing.T) {
		var arr []OptionInt
		cond := &OptionInt{
			Number: optional.Some[int64](123),
		}
		err := db.GetEngine(t.Context()).Find(&arr, cond)
		require.NoError(t, err)
		require.Len(t, arr, 1)
		v := arr[0]
		has, value := v.Number.Get()
		assert.True(t, has)
		assert.EqualValues(t, 123, value)
	})
}

func TestOptionFieldString(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	type OptionString struct {
		ID     int64 `xorm:"pk autoincr"`
		String optional.Option[string]
	}
	assert.NoError(t, db.GetEngine(t.Context()).Sync(new(OptionString)))

	t.Run("insert null, read back", func(t *testing.T) {
		null := OptionString{
			String: optional.None[string](),
		}
		cnt, err := db.GetEngine(t.Context()).Insert(&null)
		require.NoError(t, err)
		assert.EqualValues(t, 1, cnt)

		{
			var read OptionString
			has, err := db.GetEngine(t.Context()).ID(null.ID).Get(&read)
			require.NoError(t, err)
			require.True(t, has)
			hasString, _ := read.String.Get()
			assert.False(t, hasString)
		}
	})

	t.Run("insert not null, read back", func(t *testing.T) {
		notNull := OptionString{
			String: optional.Some("hello"),
		}

		cnt, err := db.GetEngine(t.Context()).Insert(&notNull)
		require.NoError(t, err)
		assert.EqualValues(t, 1, cnt)
		{
			var read OptionString
			has, err := db.GetEngine(t.Context()).ID(notNull.ID).Get(&read)
			require.NoError(t, err)
			require.True(t, has)
			hasString, str := read.String.Get()
			assert.True(t, hasString)
			assert.Equal(t, "hello", str)
		}
	})

	t.Run("read multiple records without filters", func(t *testing.T) {
		var arr []OptionString
		err := db.GetEngine(t.Context()).Find(&arr)
		require.NoError(t, err)
		assert.Len(t, arr, 2)
	})

	t.Run("read multiple records with bean filters", func(t *testing.T) {
		var arr []OptionString
		cond := &OptionString{
			String: optional.Some("hello"),
		}
		err := db.GetEngine(t.Context()).Find(&arr, cond)
		require.NoError(t, err)
		require.Len(t, arr, 1)
		v := arr[0]
		has, value := v.String.Get()
		assert.True(t, has)
		assert.Equal(t, "hello", value)
	})
}

func TestOptionFieldIntrospection(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	type OptionIntrospectInt struct {
		ID           int64 `xorm:"pk autoincr"`
		OptionNumber optional.Option[int64]
		NormalNumber int64
	}
	assert.NoError(t, db.GetEngine(t.Context()).Sync(new(OptionIntrospectInt)))

	schema, err := db.GetEngine(t.Context()).NoAutoTime().Engine().TableInfo(&OptionIntrospectInt{})
	require.NoError(t, err)

	var optionColumn, normalColumn *schemas.Column
	for _, c := range schema.Columns() {
		switch c.Name {
		case "option_number":
			optionColumn = c
		case "normal_number":
			normalColumn = c
		}
	}
	require.NotNil(t, optionColumn)
	require.NotNil(t, normalColumn)

	assert.Equal(t, normalColumn.TableName, optionColumn.TableName, "field TableName")
	assert.Equal(t, normalColumn.SQLType, optionColumn.SQLType, "field SQLType")
	assert.Equal(t, normalColumn.IsJSON, optionColumn.IsJSON, "field IsJSON")
	assert.Equal(t, normalColumn.Length, optionColumn.Length, "field Length")
	assert.Equal(t, normalColumn.Length2, optionColumn.Length2, "field Length2")
	assert.Equal(t, normalColumn.Nullable, optionColumn.Nullable, "field Nullable")
	assert.Equal(t, normalColumn.Default, optionColumn.Default, "field Default")
	assert.Equal(t, normalColumn.Indexes, optionColumn.Indexes, "field Indexes")
	assert.Equal(t, normalColumn.IsPrimaryKey, optionColumn.IsPrimaryKey, "field IsPrimaryKey")
	assert.Equal(t, normalColumn.IsAutoIncrement, optionColumn.IsAutoIncrement, "field IsAutoIncrement")
	assert.Equal(t, normalColumn.MapType, optionColumn.MapType, "field MapType")
	assert.Equal(t, normalColumn.IsCreated, optionColumn.IsCreated, "field IsCreated")
	assert.Equal(t, normalColumn.IsUpdated, optionColumn.IsUpdated, "field IsUpdated")
	assert.Equal(t, normalColumn.IsDeleted, optionColumn.IsDeleted, "field IsDeleted")
	assert.Equal(t, normalColumn.IsCascade, optionColumn.IsCascade, "field IsCascade")
	assert.Equal(t, normalColumn.IsVersion, optionColumn.IsVersion, "field IsVersion")
	assert.Equal(t, normalColumn.DefaultIsEmpty, optionColumn.DefaultIsEmpty, "field DefaultIsEmpty")
	assert.Equal(t, normalColumn.EnumOptions, optionColumn.EnumOptions, "field EnumOptions")
	assert.Equal(t, normalColumn.SetOptions, optionColumn.SetOptions, "field SetOptions")
	assert.Equal(t, normalColumn.DisableTimeZone, optionColumn.DisableTimeZone, "field DisableTimeZone")
	assert.Equal(t, normalColumn.TimeZone, optionColumn.TimeZone, "field TimeZone")
	assert.Equal(t, normalColumn.Comment, optionColumn.Comment, "field Comment")
	assert.Equal(t, normalColumn.Collation, optionColumn.Collation, "field Collation")
}
