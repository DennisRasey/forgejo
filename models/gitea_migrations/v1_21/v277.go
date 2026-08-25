// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"code.forgejo.org/xorm/xorm"
)

func AddIndexToIssueUserIssueID(x *xorm.Engine) error {
	type IssueUser struct {
		IssueID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(IssueUser))
}
