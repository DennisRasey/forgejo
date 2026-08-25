// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"fmt"
	"testing"

	migration_tests "forgejo.org/models/gitea_migrations/test"
	"forgejo.org/modules/test"

	"code.forgejo.org/xorm/xorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noOpMigration(x *xorm.Engine) error {
	return nil
}

func nilMigration() *Migration {
	return &Migration{
		Description: "nothing",
		Upgrade:     noOpMigration,
	}
}

func TestRegisterMigration(t *testing.T) {
	resetMigrations()

	defer test.MockVariableValue(&getMigrationFilename, func() string {
		return "some-path/v99b_neat_migration.go"
	})()

	t.Run("migrationResolutionComplete", func(t *testing.T) {
		defer test.MockVariableValue(&migrationResolutionComplete, true)()
		assert.PanicsWithValue(t, "attempted to register migration from some-path/v99b_neat_migration.go after migration resolution is already complete", func() {
			registerMigration(nilMigration())
		})
	})

	for _, fn := range []string{
		"v99b_neat_migration.go", // no leading path
		"vb_neat_migration.go",   // no version number
		"v12_neat_migration.go",  // no migration group letter
		"v12a-neat-migration.go", // no underscore
		"v12a.go",                // no descriptive identifier
	} {
		t.Run(fmt.Sprintf("bad name - %s", fn), func(t *testing.T) {
			defer test.MockVariableValue(&getMigrationFilename, func() string {
				return fn
			})()
			assert.PanicsWithValue(t, fmt.Sprintf("registerMigration must be invoked from a file matching migrationFilenameRegex, but was invoked from %q", fn), func() {
				registerMigration(nilMigration())
			})
		})
	}

	registerMigration(nilMigration())
	found := false
	for _, m := range rawMigrations {
		if m.id == "v99b_neat_migration" {
			found = true
		}
	}
	require.True(t, found, "found registered migration")
}

func TestResolveMigrations(t *testing.T) {
	t.Run("duplicate migration IDs", func(t *testing.T) {
		resetMigrations()
		defer test.MockVariableValue(&getMigrationFilename, func() string {
			return "some-path/v99b_neat_migration.go"
		})()
		registerMigration(nilMigration())
		registerMigration(nilMigration())

		assert.PanicsWithValue(t, "migration id is duplicated: \"v99b_neat_migration\"", func() {
			resolveMigrations()
		})
	})

	t.Run("success", func(t *testing.T) {
		resetMigrations()
		defer test.MockVariableValue(&getMigrationFilename, func() string {
			return "some-path/v99b_neat_migration.go"
		})()
		registerMigration(nilMigration())

		defer test.MockVariableValue(&getMigrationFilename, func() string {
			return "some-path/v77a_neat_migration.go"
		})()
		registerMigration(nilMigration())

		resolveMigrations()

		assert.True(t, migrationResolutionComplete, "migration resolution complete")
		assert.Contains(t, inMemoryMigrationIDs, "v77a_neat_migration")
		assert.Contains(t, inMemoryMigrationIDs, "v99b_neat_migration")
		require.Len(t, orderedMigrations, 2)
		assert.Equal(t, "v77a_neat_migration", orderedMigrations[0].id)
		assert.Equal(t, "v99b_neat_migration", orderedMigrations[1].id)
	})
}

func TestGetInDBMigrationIDs(t *testing.T) {
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(ForgejoMigration))
	defer deferable()
	require.NotNil(t, x)

	migrationIDs, err := getInDBMigrationIDs(x)
	require.NoError(t, err)
	require.NotNil(t, migrationIDs)
	assert.Empty(t, migrationIDs)

	_, err = x.Insert(&ForgejoMigration{ID: "v77a_neat_migration"})
	require.NoError(t, err)
	_, err = x.Insert(&ForgejoMigration{ID: "v99b_neat_migration"})
	require.NoError(t, err)

	migrationIDs, err = getInDBMigrationIDs(x)
	require.NoError(t, err)
	require.NotNil(t, migrationIDs)
	assert.Len(t, migrationIDs, 2)
	assert.Contains(t, migrationIDs, "v77a_neat_migration")
	assert.Contains(t, migrationIDs, "v99b_neat_migration")
}

func TestEnsureUpToDate(t *testing.T) {
	tests := []struct {
		desc     string
		inMemory []string
		inDB     []string
		err      string
	}{
		{
			desc:     "up-to-date",
			inMemory: []string{"v77a_neat_migration"},
			inDB:     []string{"v77a_neat_migration"},
		},
		{
			desc:     "invalid-migration",
			inMemory: []string{},
			inDB:     []string{"v77a_neat_migration"},
			err:      "current Forgejo database has migration(s) v77a_neat_migration applied, which are not registered migrations",
		},
		{
			desc:     "missing-migration",
			inMemory: []string{"v77a_neat_migration"},
			inDB:     []string{},
			err:      "current Forgejo database is missing migration(s) v77a_neat_migration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			resetMigrations()
			x, deferable := migration_tests.PrepareTestEnv(t, 0, new(ForgejoMigration))
			defer deferable()
			require.NotNil(t, x)

			for _, inMemory := range tc.inMemory {
				defer test.MockVariableValue(&getMigrationFilename, func() string {
					return fmt.Sprintf("some-path/%s.go", inMemory)
				})()
				registerMigration(nilMigration())
			}
			for _, inDB := range tc.inDB {
				x.Insert(&ForgejoMigration{ID: inDB})
			}

			err := EnsureUpToDate(x)
			if tc.err == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.err)
			}
		})
	}
}

func TestMigrate(t *testing.T) {
	resetMigrations()
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(ForgejoMigration))
	defer deferable()
	require.NotNil(t, x)

	v77aRun := false
	defer test.MockVariableValue(&getMigrationFilename, func() string {
		return "some-path/v77a_neat_migration.go"
	})()
	registerMigration(&Migration{
		Description: "nothing",
		Upgrade: func(x *xorm.Engine) error {
			v77aRun = true
			return nil
		},
	})
	// v77a_neat_migration will already be marked as already run
	_, err := x.Insert(&ForgejoMigration{ID: "v77a_neat_migration"})
	require.NoError(t, err)

	v99bRun := false
	defer test.MockVariableValue(&getMigrationFilename, func() string {
		return "some-path/v99b_neat_migration.go"
	})()
	registerMigration(&Migration{
		Description: "nothing",
		Upgrade: func(x *xorm.Engine) error {
			v99bRun = true
			type ForgejoMagicFunctionality struct {
				ID   int64 `xorm:"pk autoincr"`
				Name string
			}
			return x.Sync(new(ForgejoMagicFunctionality))
		},
	})

	v99cRun := false
	defer test.MockVariableValue(&getMigrationFilename, func() string {
		return "some-path/v99c_neat_migration.go"
	})()
	registerMigration(&Migration{
		Description: "nothing",
		Upgrade: func(x *xorm.Engine) error {
			v99cRun = true
			type ForgejoMagicFunctionality struct {
				NewField string
			}
			return x.Sync(new(ForgejoMagicFunctionality))
		},
	})

	err = Migrate(x, false)
	require.NoError(t, err)

	assert.False(t, v77aRun, "v77aRun") // was already marked as run in the DB so shouldn't have run again
	assert.True(t, v99bRun, "v99bRun")
	assert.True(t, v99cRun, "v99cRun")
	migrationIDs, err := getInDBMigrationIDs(x)
	require.NoError(t, err)
	assert.Contains(t, migrationIDs, "v77a_neat_migration")
	assert.Contains(t, migrationIDs, "v99b_neat_migration")
	assert.Contains(t, migrationIDs, "v99c_neat_migration")

	// should be able to query all three of the fields from this table created, verifying both migrations creating the
	// table and adding a column were run
	rec := make([]map[string]any, 0)
	err = x.Cols("id", "name", "new_field").Table("forgejo_magic_functionality").Find(&rec)
	assert.NoError(t, err)
}

func TestMigrateFreshDB(t *testing.T) {
	resetMigrations()
	x, deferable := migration_tests.PrepareTestEnv(t, 0, new(ForgejoMigration))
	defer deferable()
	require.NotNil(t, x)

	v77aRun := false
	defer test.MockVariableValue(&getMigrationFilename, func() string {
		return "some-path/v77a_neat_migration.go"
	})()
	registerMigration(&Migration{
		Description: "nothing",
		Upgrade: func(x *xorm.Engine) error {
			v77aRun = true
			return nil
		},
	})

	v99bRun := false
	defer test.MockVariableValue(&getMigrationFilename, func() string {
		return "some-path/v99b_neat_migration.go"
	})()
	registerMigration(&Migration{
		Description: "nothing",
		Upgrade: func(x *xorm.Engine) error {
			v99bRun = true
			type ForgejoMagicFunctionality struct {
				ID   int64 `xorm:"pk autoincr"`
				Name string
			}
			return x.Sync(new(ForgejoMagicFunctionality))
		},
	})

	v99cRun := false
	defer test.MockVariableValue(&getMigrationFilename, func() string {
		return "some-path/v99c_neat_migration.go"
	})()
	registerMigration(&Migration{
		Description: "nothing",
		Upgrade: func(x *xorm.Engine) error {
			v99cRun = true
			type ForgejoMagicFunctionality struct {
				NewField string
			}
			return x.Sync(new(ForgejoMagicFunctionality))
		},
	})

	err := Migrate(x, true)
	require.NoError(t, err)

	assert.False(t, v77aRun, "v77aRun") // none should be run due to freshDB flag
	assert.False(t, v99bRun, "v99bRun")
	assert.False(t, v99cRun, "v99cRun")
	migrationIDs, err := getInDBMigrationIDs(x)
	require.NoError(t, err)
	assert.Contains(t, migrationIDs, "v77a_neat_migration")
	assert.Contains(t, migrationIDs, "v99b_neat_migration")
	assert.Contains(t, migrationIDs, "v99c_neat_migration")
}
