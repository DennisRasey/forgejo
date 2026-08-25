// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import (
	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

func AddForeignKeysAccess(x *xorm.Engine) error {
	type Access struct {
		UserID int64 `xorm:"UNIQUE(s) REFERENCES(user, id)"`
		RepoID int64 `xorm:"UNIQUE(s) REFERENCES(repository, id)"`
	}
	return syncForeignKeyWithDelete(x,
		new(Access),
		builder.Or(
			builder.Expr("NOT EXISTS (SELECT id FROM repository WHERE repository.id = access.repo_id)"),
			builder.Expr("NOT EXISTS (SELECT id FROM `user` WHERE `user`.id = access.user_id)"),
		),
	)
}
