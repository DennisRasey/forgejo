// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"io/fs"

	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
)

type ActionUser struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX UNIQUE(action_user_index) REFERENCES(user, id)"`
	RepoID int64 `xorm:"INDEX UNIQUE(action_user_index) REFERENCES(repository, id)"`

	TrustedWithPullRequests bool

	LastAccess timeutil.TimeStamp `xorm:"INDEX"`
}

func testSyncBad1(x *xorm.Engine) error {
	// ruleid:xorm-sync-missing-ignore-drop-indices
	return x.Sync(new(ActionUser))
}

func testSyncBad2(x *xorm.Engine) error {
	// ruleid:xorm-sync-missing-ignore-drop-indices
	_, err = x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: false}, bean)
	return err
}

func testSyncGood1(x *xorm.Engine) error {
	// ok:xorm-sync-missing-ignore-drop-indices
	_, err = x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, bean)
	return err
}

func testSyncGood2(x *fs.File) error {
	// ok:xorm-sync-missing-ignore-drop-indices
	_, err = x.Sync()
	return err
}
