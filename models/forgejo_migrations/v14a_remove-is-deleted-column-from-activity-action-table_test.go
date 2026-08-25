// Copyright 2025 The Forgejo Authors.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"testing"

	migration_tests "forgejo.org/models/gitea_migrations/test"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm/schemas"
	"github.com/stretchr/testify/require"
)

func Test_removeIsDeletedColumnFromActivityActionTable(t *testing.T) {
	type Action struct {
		ID          int64 `xorm:"pk autoincr"`
		UserID      int64 `xorm:"INDEX"` // Receiver user id.
		ActUserID   int64 // Action user id.
		RepoID      int64
		CommentID   int64 `xorm:"INDEX"`
		IsDeleted   bool  `xorm:"NOT NULL DEFAULT false"`
		RefName     string
		IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
		Content     string             `xorm:"TEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
	}

	// Prepare TestEnv
	x, deferable := migration_tests.PrepareTestEnv(t, 0,
		new(Action),
	)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// test for expected results
	getColumn := func(tn, co string) *schemas.Column {
		tables, err := x.DBMetas()
		require.NoError(t, err)
		var table *schemas.Table
		for _, elem := range tables {
			if elem.Name == tn {
				table = elem
				break
			}
		}
		return table.GetColumn(co)
	}

	require.NotNil(t, getColumn("action", "is_deleted"))
	_, err := x.Table("action").Count()
	require.NoError(t, err)

	require.NoError(t, removeIsDeletedColumnFromActivityActionTable(x))

	require.Nil(t, getColumn("action", "is_deleted"))
	cnt2, err := x.Table("action").Count()
	require.NoError(t, err)
	require.Equal(t, int64(0), cnt2)
}
