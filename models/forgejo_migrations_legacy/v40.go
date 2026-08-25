// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddIndexForReleaseSha1(x *xorm.Engine) error {
	type Release struct {
		Sha1 string `xorm:"INDEX VARCHAR(64)"`
	}
	return x.Sync(new(Release))
}
