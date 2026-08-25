// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddExternalURLColumnToAttachmentTable(x *xorm.Engine) error {
	type Attachment struct {
		ID          int64 `xorm:"pk autoincr"`
		ExternalURL string
	}
	return x.Sync(new(Attachment))
}
