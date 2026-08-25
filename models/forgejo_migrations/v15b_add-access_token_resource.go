// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add access_token_resource table",
		Upgrade:     addAccessTokenResource,
	})
}

func addAccessTokenResource(x *xorm.Engine) error {
	type AccessTokenResourceRepo struct {
		ID      int64 `xorm:"pk autoincr"`
		TokenID int64 `xorm:"NOT NULL REFERENCES(access_token, id)"` // needs to be shortened from "AccessTokenID" for the index to fit MySQL table identifier length restrictions
		RepoID  int64 `xorm:"NOT NULL REFERENCES(repository, id)"`

		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(AccessTokenResourceRepo))
	return err
}
