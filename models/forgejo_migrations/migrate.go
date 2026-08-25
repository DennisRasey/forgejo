// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"forgejo.org/modules/container"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
	"code.forgejo.org/xorm/xorm/names"
)

// ForgejoMigration table contains a record of migrations applied to the database.  (Note that there are older
// migrations in the forgejo_version table from before this table was introduced, and the `version` table from Gitea
// migrations). Each record in this table represents one successfully completed migration which was completed at the
// `CreatedUnix` time.
type ForgejoMigration struct {
	ID          string             `xorm:"pk"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

type Migration struct {
	Description string                   // short plaintext explanation of the migration
	Upgrade     func(*xorm.Engine) error // perform the migration

	id string // unique migration identifier
}

var (
	rawMigrations          []*Migration
	migrationFilenameRegex = regexp.MustCompile(`/(?P<migration_group>v[0-9]+[a-z])_(?P<migration_id>[^/]+)\.go$`)
)

var getMigrationFilename = func() string {
	_, migrationFilename, _, _ := runtime.Caller(2)
	return migrationFilename
}

func registerMigration(migration *Migration) {
	migrationFilename := getMigrationFilename()

	if migrationResolutionComplete {
		panic(fmt.Sprintf("attempted to register migration from %s after migration resolution is already complete", migrationFilename))
	}

	matches := migrationFilenameRegex.FindStringSubmatch(migrationFilename)
	if len(matches) == 0 {
		panic(fmt.Sprintf("registerMigration must be invoked from a file matching migrationFilenameRegex, but was invoked from %q", migrationFilename))
	}
	migration.id = fmt.Sprintf("%s_%s", matches[1], matches[2]) // this just rebuilds the filename, but guarantees that the regex applied for consistent naming

	rawMigrations = append(rawMigrations, migration)
}

// For testing only
func resetMigrations() {
	rawMigrations = nil
	orderedMigrations = nil
	migrationResolutionComplete = false
	inMemoryMigrationIDs = nil
}

var (
	migrationResolutionComplete = false
	inMemoryMigrationIDs        container.Set[string]
	orderedMigrations           []*Migration
)

func resolveMigrations() {
	if migrationResolutionComplete {
		return
	}

	inMemoryMigrationIDs = make(container.Set[string])
	for _, m := range rawMigrations {
		if inMemoryMigrationIDs.Contains(m.id) {
			// With the filename-based migration ID this shouldn't be possible, but a bit of a sanity check..
			panic(fmt.Sprintf("migration id is duplicated: %q", m.id))
		}
		inMemoryMigrationIDs.Add(m.id)
	}

	orderedMigrations = slices.Clone(rawMigrations)
	slices.SortFunc(orderedMigrations, func(m1, m2 *Migration) int {
		return strings.Compare(m1.id, m2.id)
	})

	migrationResolutionComplete = true
}

func getInDBMigrationIDs(x *xorm.Engine) (container.Set[string], error) {
	var inDBMigrations []ForgejoMigration
	err := x.Find(&inDBMigrations)
	if err != nil {
		return nil, err
	}

	inDBMigrationIDs := make(container.Set[string], len(inDBMigrations))
	for _, inDB := range inDBMigrations {
		inDBMigrationIDs.Add(inDB.ID)
	}

	return inDBMigrationIDs, nil
}

// EnsureUpToDate will check if the Forgejo database is at the correct version.
func EnsureUpToDate(x *xorm.Engine) error {
	resolveMigrations()

	inDBMigrationIDs, err := getInDBMigrationIDs(x)
	if err != nil {
		return err
	}

	// invalidMigrations are those that are in the database, but aren't registered.
	invalidMigrations := inDBMigrationIDs.Difference(inMemoryMigrationIDs)
	if len(invalidMigrations) > 0 {
		return fmt.Errorf("current Forgejo database has migration(s) %s applied, which are not registered migrations", strings.Join(invalidMigrations.Slice(), ", "))
	}

	// unappliedMigrations are those that haven't yet been applied, but seem valid
	unappliedMigrations := inMemoryMigrationIDs.Difference(inDBMigrationIDs)
	if len(unappliedMigrations) > 0 {
		return fmt.Errorf(`current Forgejo database is missing migration(s) %s. Please run "forgejo [--config /path/to/app.ini] migrate" to update the database version`, strings.Join(unappliedMigrations.Slice(), ", "))
	}

	return nil
}

func recordMigrationComplete(x *xorm.Engine, migration *Migration) error {
	affected, err := x.Insert(&ForgejoMigration{ID: migration.id})
	if err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("migration[%s]: failed to insert into DB, %d records affected", migration.id, affected)
	}
	return nil
}

// Migrate Forgejo database to current version.
func Migrate(x *xorm.Engine, freshDB bool) error {
	resolveMigrations()

	// Set a new clean the default mapper to GonicMapper as that is the default for .
	x.SetMapper(names.GonicMapper{})
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ForgejoMigration)); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	inDBMigrationIDs, err := getInDBMigrationIDs(x)
	if err != nil {
		return err
	} else if len(inDBMigrationIDs) == 0 && freshDB {
		// During startup on a new, empty database, and during integration tests, we rely only on `SyncAllTables` to
		// create the DB schema.  No migrations can be applied because `SyncAllTables` occurs later in the
		// initialization cycle.  We mark all migrations as complete up to this point and only run future migrations.
		for _, migration := range orderedMigrations {
			err := recordMigrationComplete(x, migration)
			if err != nil {
				return err
			}
		}
		inDBMigrationIDs, err = getInDBMigrationIDs(x)
		if err != nil {
			return err
		}
	} else if freshDB {
		return fmt.Errorf("unexpected state: migrator called with freshDB=true, but existing migrations in DB %#v", inDBMigrationIDs)
	}

	// invalidMigrations are those that are in the database, but aren't registered.
	invalidMigrations := inDBMigrationIDs.Difference(inMemoryMigrationIDs)
	if len(invalidMigrations) > 0 {
		// Downgrading Forgejo's database version not supported
		msg := fmt.Sprintf("Your Forgejo database has %d migration(s) (%s) for a newer version of Forgejo, you cannot use the newer database for this old Forgejo release.", len(invalidMigrations), strings.Join(invalidMigrations.Slice(), ", "))
		msg += "\nForgejo will exit to keep your database safe and unchanged. Please use the correct Forgejo release, do not change the migration version manually (incorrect manual operation may cause data loss)."
		if !setting.IsProd {
			msg += "\nIf you are in development and know what you're doing, you can remove the migration records from the forgejo_migration table.  The affect of those migrations will still be present."
			quoted := slices.Clone(invalidMigrations.Slice())
			for i, s := range quoted {
				quoted[i] = "'" + s + "'"
			}
			msg += fmt.Sprintf("\n  DELETE FROM forgejo_migration WHERE id IN (%s)", strings.Join(quoted, ", "))
		}
		_, _ = fmt.Fprintln(os.Stderr, msg)
		log.Fatal(msg)
		return nil
	}

	// unappliedMigrations are those that are registered but haven't been applied.
	unappliedMigrations := inMemoryMigrationIDs.Difference(inDBMigrationIDs)
	for _, migration := range orderedMigrations {
		if !unappliedMigrations.Contains(migration.id) {
			continue
		}

		log.Info("Migration[%s]: %s", migration.id, migration.Description)

		// Reset the mapper between each migration - migrations are not supposed to depend on each other
		x.SetMapper(names.GonicMapper{})
		if err = migration.Upgrade(x); err != nil {
			return fmt.Errorf("migration[%s]: %s failed: %w", migration.id, migration.Description, err)
		}

		err := recordMigrationComplete(x, migration)
		if err != nil {
			return err
		}
	}

	return nil
}
