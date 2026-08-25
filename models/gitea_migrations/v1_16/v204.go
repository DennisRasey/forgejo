// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import "code.forgejo.org/xorm/xorm"

func AddSSHKeyIsVerified(x *xorm.Engine) error {
	type PublicKey struct {
		Verified bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(PublicKey))
}
