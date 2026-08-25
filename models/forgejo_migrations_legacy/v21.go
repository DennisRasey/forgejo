// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddSSHKeypairToPushMirror(x *xorm.Engine) error {
	type PushMirror struct {
		ID         int64  `xorm:"pk autoincr"`
		PublicKey  string `xorm:"VARCHAR(100)"`
		PrivateKey []byte `xorm:"BLOB"`
	}

	return x.Sync(&PushMirror{})
}
