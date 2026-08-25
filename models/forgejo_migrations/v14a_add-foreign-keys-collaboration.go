// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

func init() {
	registerMigration(&Migration{
		Description: "add foreign keys to collaboration, repo_id & user_id",
		Upgrade:     addForeignKeysCollaboration,
	})
}

func addForeignKeysCollaboration(x *xorm.Engine) error {
	type Collaboration struct {
		RepoID int64 `xorm:"UNIQUE(s) INDEX NOT NULL REFERENCES(repository, id)"`
		UserID int64 `xorm:"UNIQUE(s) INDEX NOT NULL REFERENCES(user, id)"`
	}
	return syncForeignKeyWithDelete(x,
		new(Collaboration),
		builder.Or(
			builder.Expr("NOT EXISTS (SELECT id FROM repository WHERE repository.id = collaboration.repo_id)"),
			builder.Expr("NOT EXISTS (SELECT id FROM `user` WHERE `user`.id = collaboration.user_id)"),
		),
	)
}
