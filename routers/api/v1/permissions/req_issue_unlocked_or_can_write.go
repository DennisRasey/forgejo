// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package permissions

import (
	"net/http"

	issues_model "forgejo.org/models/issues"
)

func ReqIssueUnlockedOrCanWrite(ctx Context, issue *issues_model.Issue) {
	if !ctx.Permission().CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}

	if issue.IsLocked && !ctx.Permission().CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Error(http.StatusForbidden, "ReqIssueUnlockedOrCanWrite", "You cannot change a locked issue.")
		return
	}
}
