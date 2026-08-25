// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"forgejo.org/modules/setting"

	"code.forgejo.org/xorm/xorm/dialects"
	"github.com/jackc/pgx/v5/stdlib"
)

func init() {
	// Register pgx-based driver as "postgresschema" for PostgreSQL with schema support
	// This wraps pgx/v5/stdlib and injects search_path configuration on connection
	// For PostgreSQL without schema, engine.go uses "pgx" directly (registered by pgx/v5/stdlib)
	drv := &postgresSchemaDriver{innerDriver: &stdlib.Driver{}}
	sql.Register("postgresschema", drv)
	dialects.RegisterDriver("postgresschema", dialects.QueryDriver("postgres"))
}

type postgresSchemaDriver struct {
	innerDriver *stdlib.Driver
}

// Open opens a new connection to the database. name is a connection string.
// This function opens the postgres connection in the default manner but immediately
// runs set_config to set the search_path appropriately
func (d *postgresSchemaDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.innerDriver.Open(name)
	if err != nil {
		return conn, err
	}

	// pgx implements ExecerContext, not the deprecated Execer interface
	execer, ok := conn.(driver.ExecerContext)
	if !ok {
		_ = conn.Close()
		return nil, errors.New("pgx driver connection does not implement ExecerContext")
	}

	_, err = execer.ExecContext(context.Background(), `SELECT set_config(
		'search_path',
		$1 || ',' || current_setting('search_path'),
		false)`, []driver.NamedValue{{Ordinal: 1, Value: setting.Database.Schema}})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}
