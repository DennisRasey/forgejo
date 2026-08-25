// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import (
	"code.forgejo.org/xorm/xorm"
)

func AddPushMirrorBranchFilter(x *xorm.Engine) error {
	type PushMirror struct {
		ID           int64  `xorm:"pk autoincr"`
		BranchFilter string `xorm:"VARCHAR(255)"`
	}
	return x.Sync2(new(PushMirror))
}
