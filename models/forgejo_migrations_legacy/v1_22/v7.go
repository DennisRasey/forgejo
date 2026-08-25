// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.forgejo.org/xorm/xorm"
)

func AddUserRepoUnitHintsSetting(x *xorm.Engine) error {
	type User struct {
		ID                  int64 `xorm:"pk autoincr"`
		EnableRepoUnitHints bool  `xorm:"NOT NULL DEFAULT true"`
	}

	return x.Sync(&User{})
}
