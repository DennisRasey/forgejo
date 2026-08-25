// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddHideArchiveLinksToRelease(x *xorm.Engine) error {
	type Release struct {
		ID               int64 `xorm:"pk autoincr"`
		HideArchiveLinks bool  `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(&Release{})
}
