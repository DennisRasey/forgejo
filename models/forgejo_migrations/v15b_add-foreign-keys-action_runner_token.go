// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

func init() {
	registerMigration(&Migration{
		Description: "add foreign keys to action_runner_token",
		Upgrade:     addForeignKeysActionRunnerToken,
	})
}

func addForeignKeysActionRunnerToken(x *xorm.Engine) error {
	type ActionRunnerToken struct {
		OwnerID int64 `xorm:"index REFERENCES(user, id)"`
		RepoID  int64 `xorm:"index REFERENCES(repository, id)"`
	}

	// With the introduction of a foreign key, owner_id & repo_id cannot be set to "0".  Runners can be registered as
	// global (owner_id = NULL, repo_id = NULL), user/org (repo_id = NULL), or repo (owner_id = NULL) and NULL values
	// now replace the '0' values.
	_, err := x.Table(&ActionRunnerToken{}).Where("owner_id = 0").Update(map[string]any{"owner_id": nil})
	if err != nil {
		return err
	}
	_, err = x.Table(&ActionRunnerToken{}).Where("repo_id = 0").Update(map[string]any{"repo_id": nil})
	if err != nil {
		return err
	}

	return syncForeignKeyWithDelete(x,
		new(ActionRunnerToken),
		builder.Or(
			builder.And(
				builder.Expr("owner_id IS NOT NULL"),
				builder.Expr("NOT EXISTS (SELECT id FROM `user` WHERE `user`.id = action_runner_token.owner_id)"),
			),
			builder.And(
				builder.Expr("repo_id IS NOT NULL"),
				builder.Expr("NOT EXISTS (SELECT id FROM repository WHERE repository.id = action_runner_token.repo_id)"),
			),
		),
	)
}
