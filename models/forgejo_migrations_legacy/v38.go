// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import (
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func AddResolvedUnixToAbuseReport(x *xorm.Engine) error {
	type AbuseReport struct {
		ID           int64              `xorm:"pk autoincr"`
		ResolvedUnix timeutil.TimeStamp `xorm:"DEFAULT NULL"`
	}

	return x.Sync(&AbuseReport{})
}
