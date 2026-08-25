// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func AddArchivedUnixColumnInLabelTable(x *xorm.Engine) error {
	type Label struct {
		ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT NULL"`
	}
	return x.Sync(new(Label))
}
