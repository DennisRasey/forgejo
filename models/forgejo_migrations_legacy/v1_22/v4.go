// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.forgejo.org/xorm/xorm"
)

func AddDefaultPermissionsToRepoUnit(x *xorm.Engine) error {
	type RepoUnit struct {
		ID                 int64
		DefaultPermissions int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(&RepoUnit{})
}
