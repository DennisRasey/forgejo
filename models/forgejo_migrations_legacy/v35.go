// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import (
	"code.forgejo.org/xorm/xorm"
)

// see https://codeberg.org/forgejo/forgejo/issues/8373
func NoopAddIndexToActionRunStopped(x *xorm.Engine) error {
	return nil
}
