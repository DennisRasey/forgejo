// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_v1_20

import (
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func AddForgejoBlockedUser(x *xorm.Engine) error {
	type ForgejoBlockedUser struct {
		ID          int64              `xorm:"pk autoincr"`
		BlockID     int64              `xorm:"index"`
		UserID      int64              `xorm:"index"`
		CreatedUnix timeutil.TimeStamp `xorm:"created"`
	}

	return x.Sync(new(ForgejoBlockedUser))
}
