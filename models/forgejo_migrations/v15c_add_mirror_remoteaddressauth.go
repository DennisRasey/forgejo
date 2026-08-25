// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "replace remote_address with encrypted_remote_address in table mirror",
		Upgrade:     addMirrorRemoteAddressAuth,
	})
}

func addMirrorRemoteAddressAuth(x *xorm.Engine) error {
	type Mirror struct {
		EncryptedRemoteAddress []byte `xorm:"BLOB NULL"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(Mirror)); err != nil {
		return err
	}
	// No data migration is necessary or desired.  `remote_address` contains sanitized URLs which don't have
	// credentials, so they can't be migrated to `encrypted_remote_address`. Instead, as this data is accessed,
	// `DecryptOrRecoverRemoteAddress` will recover the fully credentialed contents of the remote address from the git
	// repo's `origin` remote address.
	_, err := x.Exec("ALTER TABLE `mirror` DROP COLUMN `remote_address`")
	return err
}
