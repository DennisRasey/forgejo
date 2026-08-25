// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitea_migrations

import (
	"testing"

	migration_tests "forgejo.org/models/gitea_migrations/test"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrations(t *testing.T) {
	defer test.MockVariableValue(&preparedMigrations, []*migration{
		{idNumber: 70},
		{idNumber: 71},
	})()
	assert.EqualValues(t, 72, calcDBVersion(preparedMigrations))
	assert.EqualValues(t, 72, ExpectedDBVersion())

	assert.EqualValues(t, 71, migrationIDNumberToDBVersion(70))

	assert.Equal(t, []*migration{{idNumber: 70}, {idNumber: 71}}, getPendingMigrations(70, preparedMigrations))
	assert.Equal(t, []*migration{{idNumber: 71}}, getPendingMigrations(71, preparedMigrations))
	assert.Equal(t, []*migration{}, getPendingMigrations(72, preparedMigrations))
}

func TestMigrateFreshDB(t *testing.T) {
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(Version))
	defer deferable()
	require.NotNil(t, x)

	err := Migrate(x)
	require.NoError(t, err)

	var versionRecords []*Version
	err = x.Find(&versionRecords)
	require.NoError(t, err)
	require.Len(t, versionRecords, 1)
	v := versionRecords[0]
	assert.EqualValues(t, 1, v.ID)
	assert.EqualValues(t, 305, v.Version)
}

func TestMigrateFailWithCorruption(t *testing.T) {
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(Version))
	defer deferable()
	require.NotNil(t, x)

	// ID != 1
	_, err := x.Insert(&Version{ID: 100, Version: 100})
	require.NoError(t, err)
	err = Migrate(x)
	require.ErrorContains(t, err, "corrupted records in the table `version`")

	// Two versions...
	_, err = x.Insert(&Version{ID: 1, Version: 1000})
	require.NoError(t, err)
	err = Migrate(x)
	require.ErrorContains(t, err, "unexpected records in the table `version`")
}
