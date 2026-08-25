// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"code.forgejo.org/xorm/xorm"
)

func AddScheduleIDForActionRun(x *xorm.Engine) error {
	type ActionRun struct {
		ScheduleID int64
	}
	return x.Sync(new(ActionRun))
}
