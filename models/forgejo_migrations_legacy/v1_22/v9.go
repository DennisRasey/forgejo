// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import "code.forgejo.org/xorm/xorm"

func AddApplyToAdminsSetting(x *xorm.Engine) error {
	type ProtectedBranch struct {
		ID            int64 `xorm:"pk autoincr"`
		ApplyToAdmins bool  `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(&ProtectedBranch{})
}
