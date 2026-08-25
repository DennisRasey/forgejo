# forgejo_migrations

Forgejo has three database migration mechanisms:

- `models/gitea_migrations`
    - Original database schema migrations from Forgejo's legacy as a fork of Gitea.
    - A linear set of migrations referenced in `models/gitea_migrations/migrations.go`, each represented by a number (eg. migration 304).
    - The current version is recorded in the database in the table `version`.
- `models/forgejo_migrations_legacy`
    - The next 50-ish database schema migrations reflecting change in Forgejo's database structure between Forgejo v7.0 and v14.0
    - A linear set of migrations referenced in `models/forgejo_migrations_legacy/migrate.go`, each represented by a number (eg. migration 43).
    - The current version is recorded in the database in the table `forgejo_version`.
- `models/forgejo_migrations`
    - The most recent database schema migrations, reflecting change in the v14.0 release cycle and onwards into the future.
    - Each migration is identified by the filename it is stored in.
    - The applied migrations are recorded in the database in the table `forgejo_migration`.

`forgejo_migrations` is designed to reduce code conflicts when multiple developers may be making schema migrations in close succession, which it does by avoiding having one code file with a long array of migrations.  Instead, each file in `models/forgejo_migrations` registers itself as a migration, and its filename indicates the order that migration will be applied.

Files in `forgejo_migrations` must:
- Define an `init` function which registers a function to be invoked for the migration.
- Follow the naming convention:
    - The letter `v`
    - A number, representing the development cycle that the migration was created in
    - A letter, indicating any required migration ordering
    - The character `_` (underscore)
    - A short descriptive identifier for the migration

For example, valid migration file names would look like this:
- `v14a_add-threaded-comments.go`
- `v14a_add-federated-emojis.go`
- `v14b_fix-threaded-comments-index.go`


## Migration Ordering

Forgejo executes registered migrations in `forgejo_migrations` in the `strings.Compare()` ordering of their filename.

There are edge cases where migrations may not be executed in this exact order:
- If a schema change is backported to an earlier Forgejo release.  For example, if a bugfix during the v15 development cycle was backported into a v14 patch release, then a migration labeled `v15a_fix-unusual-data-corruption.go` could be applied during a v14 software upgrade.  In the future when a v15 software release occurs, that migration will be identified as already applied and will be skipped.
- If a developer working on Forgejo switches between different branches with different schema migrations.
- If the contents of the `forgejo_migrations` database table are changed outside of Forgejo modifying it.


## Creating a new Migration

First, determine the filename for your migration.  In general, you create a new migration by starting a file with the same prefix as the most recent migration present.  If `v14a_add-forgejo-migrations-table.go` was the last file, most of the time you can create your migration with the same `v14a_...` prefix.

There are two exceptions:
- After the release branch is cut for a release, increment the version in the migration.  If v14 was cut, you would start `v15a_...` as the next migration.
- If your migration requires that an earlier migration is complete first, you would increment the letter in the prefix.  If you were modifying the table created by `v14a_add-forgejo-migrations-table.go`, then you would name your migration `v14b_...`.

Once you've determined the migration filename, then you can copy this template into the file:

```go
// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "short sentence describing this migration",
		Upgrade:     myMigrationFunction, // rename
	})
}

func myMigrationFunction(x *xorm.Engine) error {
    // add migration logic here
    //
    // to prevent `make watch` from recording this migration as done when it
    // isn't authored yet, returh an error until the implementation is done
    return errors.New("not implemented yet")
}
```

And now it's up to you to write the contents of your migration function.


## Development Notes

Once migrations are executed, a record of their execution is stored in the database table `forgejo_migration`.

```sql
=> SELECT * FROM forgejo_migration;
                id                 | created_unix
-----------------------------------+--------------
 v14a_add-forgejo-migrations-table |   1760402451
 v14a_example-other-migration      |   1760402453
 v14b_another-example              |   1760402455
 v15a_add-something-cool           |   1760402456
 v15a_another-example-again        |   1760402457
```

If your migration successfully executes once, it will be recorded in this table and it will never execute again, even if you change the migration code.  It is common during development to need to re-run a migration, in which case you can delete the record that you're working on developing.  The migration will be re-run as soon as the Forgejo server is restarted:

```sql
=> DELETE FROM forgejo_migration WHERE id = 'v15a_another-example-again';
```
