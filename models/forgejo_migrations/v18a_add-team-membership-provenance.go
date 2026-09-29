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
		Description: "add membership provenance fields to TeamUser",
		Upgrade:     addMembershipProvenanceToTeamUser,
	})
}

func addMembershipProvenanceToTeamUser(x *xorm.Engine) error {
	type MembershipReason int
	type TeamUser struct {
		CreatedUnix            optional.Option[timeutil.TimeStamp] `xorm:"created_unix"`
		Reason                 MembershipReason                    `xorm:"NOT NULL DEFAULT 0"`
		CreatedByUserID        optional.Option[int64]              `xorm:"index REFERENCES(user, id)"`
		CreatedByLoginSourceID optional.Option[int64]              `xorm:"INDEX REFERENCES(login_source, id)"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(TeamUser))
	return err
}
