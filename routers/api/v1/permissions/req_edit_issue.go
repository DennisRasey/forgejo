// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package permissions

import (
	"fmt"
	"net/http"
	"strings"

	issues_model "forgejo.org/models/issues"
	api "forgejo.org/modules/structs"
)

func ReqEditIssue(ctx Context, issue *issues_model.Issue, opt *api.EditIssueOption) {
	if !ctx.Permission().CanReadIssuesOrPulls(issue.IsPull) {
		ctx.NotFound()
		return
	}

	errors := []string{}

	isOwner := ctx.Permission().IsOwner() || IsUserSiteAdmin(ctx)
	if opt.Updated != nil && !isOwner {
		errors = append(errors, "updated_at")
	}

	canWrite := ctx.Permission().CanWriteIssuesOrPulls(issue.IsPull)
	if opt.Assignees != nil && !canWrite {
		errors = append(errors, "assignees")
	}
	if opt.Milestone != nil && !canWrite {
		errors = append(errors, "milestone")
	}
	if opt.Deadline != nil && !canWrite {
		errors = append(errors, "due_date")
	}
	if opt.RemoveDeadline != nil && !canWrite {
		errors = append(errors, "unset_due_date")
	}

	isUnlockedAndPoster := !issue.IsLocked && issue.IsPoster(ctx.Doer().ID)
	messageUnlockedAndPoster := func(field string) string {
		if issue.IsLocked {
			return fmt.Sprintf("%s(locked)", field)
		}
		return field
	}
	if len(opt.Title) > 0 && !isUnlockedAndPoster && !canWrite {
		errors = append(errors, messageUnlockedAndPoster("title"))
	}
	if opt.Body != nil && !isUnlockedAndPoster && !canWrite {
		errors = append(errors, messageUnlockedAndPoster("body"))
	}
	if opt.Ref != nil && !isUnlockedAndPoster && !canWrite {
		errors = append(errors, messageUnlockedAndPoster("ref"))
	}
	if opt.State != nil && !isUnlockedAndPoster && !canWrite {
		errors = append(errors, messageUnlockedAndPoster("state"))
	}

	if len(errors) > 0 {
		ctx.Error(http.StatusForbidden, "ReqEditIssue", fmt.Sprintf("No permission to edit the following issue fields: %s", strings.Join(errors, ",")))
	}
}
