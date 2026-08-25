// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.forgejo.org/xorm/xorm"
)

func AddPronounsToUser(x *xorm.Engine) error {
	type User struct {
		ID       int64 `xorm:"pk autoincr"`
		Pronouns string
	}

	return x.Sync(&User{})
}
