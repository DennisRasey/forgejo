// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.forgejo.org/xorm/xorm"
)

func AddIgnoreStaleApprovalsColumnToProtectedBranchTable(x *xorm.Engine) error {
	type ProtectedBranch struct {
		IgnoreStaleApprovals bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, new(ProtectedBranch))
	return err
}
