// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"forgejo.org/modules/optional"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "add expiry to team_invite",
		Upgrade:     addTeamInviteExpiryDate,
	})
}

func addTeamInviteExpiryDate(x *xorm.Engine) error {
	// the expiry date is set to None if the invite doesn't expire
	type TeamInvite struct {
		ExpiryDate optional.Option[timeutil.TimeStamp] `xorm:"expiry_unix"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(TeamInvite))
	return err
}
