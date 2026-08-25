// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

func AddNotifyEmailToActionRun(x *xorm.Engine) error {
	type ActionRun struct {
		ID          int64
		NotifyEmail bool
	}
	return x.Sync(new(ActionRun))
}
