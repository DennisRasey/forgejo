// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddPurposeToForgejoAuthToken(x *xorm.Engine) error {
	type ForgejoAuthToken struct {
		ID      int64  `xorm:"pk autoincr"`
		Purpose string `xorm:"NOT NULL DEFAULT 'long_term_authorization'"`
	}
	if err := x.Sync(new(ForgejoAuthToken)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE `forgejo_auth_token` SET purpose = 'long_term_authorization' WHERE purpose = ''")
	return err
}
