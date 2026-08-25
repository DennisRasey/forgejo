// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import "code.forgejo.org/xorm/xorm"

func DropWronglyCreatedTable(x *xorm.Engine) error {
	return x.DropTables("o_auth2_application")
}
