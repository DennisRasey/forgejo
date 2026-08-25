// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"forgejo.org/models/gitea_migrations/base"

	"code.forgejo.org/xorm/xorm"
)

func RecreateIssueResourceIndexTable(x *xorm.Engine) error {
	type IssueIndex struct {
		GroupID  int64 `xorm:"pk"`
		MaxIndex int64 `xorm:"index"`
	}

	return base.RecreateTables(new(IssueIndex))(x)
}
