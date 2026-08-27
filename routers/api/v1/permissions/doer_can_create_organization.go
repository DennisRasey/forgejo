// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package permissions

import (
	"net/http"
)

func DoerCanCreateOrganization(ctx Context) {
	if !ctx.Doer().CanCreateOrganization() {
		ctx.Error(http.StatusForbidden, "Create organization not allowed", ctx.Doer().Name)
	}
}
