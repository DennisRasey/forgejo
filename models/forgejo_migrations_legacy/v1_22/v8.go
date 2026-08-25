// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"strings"

	"code.forgejo.org/xorm/xorm"
)

func RemoveSSHSignaturesFromReleaseNotes(x *xorm.Engine) error {
	type Release struct {
		ID   int64  `xorm:"pk autoincr"`
		Note string `xorm:"TEXT"`
	}

	if err := x.Sync(&Release{}); err != nil {
		return err
	}

	var releaseNotes []struct {
		ID   int64
		Note string
	}

	if err := x.Table("release").Where("note LIKE '%-----BEGIN SSH SIGNATURE-----%'").Find(&releaseNotes); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	for _, release := range releaseNotes {
		idx := strings.LastIndex(release.Note, "-----BEGIN SSH SIGNATURE-----")
		if idx == -1 {
			continue
		}
		release.Note = release.Note[:idx]
		_, err := sess.Exec("UPDATE `release` SET note = ? WHERE id = ?", release.Note, release.ID)
		if err != nil {
			return err
		}
	}

	return sess.Commit()
}
