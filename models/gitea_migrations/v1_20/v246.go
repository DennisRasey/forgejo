// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20

import (
	"code.forgejo.org/xorm/xorm"
)

func AddNewColumnForProject(x *xorm.Engine) error {
	type Project struct {
		OwnerID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(Project))
}
