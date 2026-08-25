// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import (
	"testing"

	migration_tests "forgejo.org/models/gitea_migrations/test"
	"forgejo.org/modules/test"

	"code.forgejo.org/xorm/xorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureUpToDate tests the behavior of EnsureUpToDate.
func TestEnsureUpToDate(t *testing.T) {
	defer test.MockVariableValue(&forgejoMigrationsEnsureUpToDate, func(x *xorm.Engine) error {
		return nil
	})()

	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(ForgejoVersion))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// Ensure error if there's no row in Forgejo Version.
	err := EnsureUpToDate(x)
	require.Error(t, err)

	// Insert 'good' Forgejo Version row.
	_, err = x.Insert(&ForgejoVersion{ID: 1, Version: ExpectedVersion()})
	require.NoError(t, err)

	err = EnsureUpToDate(x)
	require.NoError(t, err)

	// Modify forgejo version to have a lower version.
	_, err = x.Exec("UPDATE `forgejo_version` SET version = ? WHERE id = 1", ExpectedVersion()-1)
	require.NoError(t, err)

	err = EnsureUpToDate(x)
	require.Error(t, err)
}

func TestMigrateFreshDB(t *testing.T) {
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(ForgejoVersion))
	defer deferable()
	require.NotNil(t, x)

	err := Migrate(x)
	require.NoError(t, err)

	var versionRecords []*ForgejoVersion
	err = x.Find(&versionRecords)
	require.NoError(t, err)
	require.Len(t, versionRecords, 1)
	v := versionRecords[0]
	assert.EqualValues(t, 1, v.ID)
	assert.EqualValues(t, 44, v.Version)
}

func TestMigrateFailWithCorruption(t *testing.T) {
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(ForgejoVersion))
	defer deferable()
	require.NotNil(t, x)

	// ID != 1
	_, err := x.Insert(&ForgejoVersion{ID: 100, Version: 100})
	require.NoError(t, err)
	err = Migrate(x)
	require.ErrorContains(t, err, "corrupted records in the table `forgejo_version`")

	// Two versions...
	_, err = x.Insert(&ForgejoVersion{ID: 1, Version: 1000})
	require.NoError(t, err)
	err = Migrate(x)
	require.ErrorContains(t, err, "unexpected records in the table `forgejo_version`")
}
