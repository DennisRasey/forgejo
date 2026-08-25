// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"code.forgejo.org/xorm/xorm"
)

func CommentTypeDeleteBranchUseOldRef(x *xorm.Engine) error {
	_, err := x.Exec("UPDATE comment SET old_ref = commit_sha, commit_sha = '' WHERE type = 11")
	return err
}
