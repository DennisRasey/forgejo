// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package permissions

import (
	issues_model "forgejo.org/models/issues"
	"forgejo.org/models/unit"
)

func MustEnableLocalIssuesIfIsIssue(ctx Context, issue *issues_model.Issue) {
	if ctx.Repository().UnitEnabled(ctx.Context(), unit.TypeIssues) {
		return
	}

	if !issue.IsPull {
		ctx.NotFound()
		return
	}
}
