// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add resource_all_owned_repositories to table access_token",
		Upgrade:     addAllOwnedRepositoriesToAccessToken,
	})
}

func addAllOwnedRepositoriesToAccessToken(x *xorm.Engine) error {
	type AccessToken struct {
		ResourceAllRepos bool `xorm:"NOT NULL DEFAULT TRUE"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(AccessToken))
	return err
}
