// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import (
	"database/sql"

	"code.forgejo.org/xorm/xorm"
)

func AddPublicKeyInformationForFederation(x *xorm.Engine) error {
	type FederationHost struct {
		KeyID     sql.NullString         `xorm:"key_id UNIQUE"`
		PublicKey sql.Null[sql.RawBytes] `xorm:"BLOB"`
	}

	err := x.Sync(&FederationHost{})
	if err != nil {
		return err
	}

	type FederatedUser struct {
		KeyID     sql.NullString         `xorm:"key_id UNIQUE"`
		PublicKey sql.Null[sql.RawBytes] `xorm:"BLOB"`
	}

	return x.Sync(&FederatedUser{})
}
