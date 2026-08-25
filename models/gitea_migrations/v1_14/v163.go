// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"forgejo.org/models/gitea_migrations/base"

	"code.forgejo.org/xorm/xorm"
)

func ConvertTopicNameFrom25To50(x *xorm.Engine) error {
	type Topic struct {
		ID          int64  `xorm:"pk autoincr"`
		Name        string `xorm:"UNIQUE VARCHAR(50)"`
		RepoCount   int
		CreatedUnix int64 `xorm:"INDEX created"`
		UpdatedUnix int64 `xorm:"INDEX updated"`
	}

	if err := x.Sync(new(Topic)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := base.LegacyRecreateTable(sess, new(Topic)); err != nil { //nolint:staticcheck
		return err
	}

	return sess.Commit()
}
