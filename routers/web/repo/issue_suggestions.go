// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"forgejo.org/models/unit"
	"forgejo.org/modules/optional"
	"forgejo.org/services/context"
	issue_service "forgejo.org/services/issue"
)

// IssueSuggestions returns a list of issue suggestions
func IssueSuggestions(ctx *context.Context) {
	keyword := ctx.FormString("q")
	isPull := ctx.FormOptionalBool("pull")

	if has, value := isPull.Get(); has {
		if !ctx.Repo.CanReadIssuesOrPulls(value) {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
	} else {
		canReadIssues := ctx.Repo.CanRead(unit.TypeIssues)
		canReadPulls := ctx.Repo.CanRead(unit.TypePullRequests)

		if !canReadPulls && !canReadIssues {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		} else if canReadPulls && !canReadIssues {
			isPull = optional.Some(true)
		} else if canReadIssues && !canReadPulls {
			isPull = optional.Some(false)
		}
	}

	suggestions, err := issue_service.GetSuggestions(ctx, ctx.Repo.Repository, isPull, keyword)
	if err != nil {
		ctx.ServerError("GetSuggestions", err)
		return
	}

	ctx.JSON(http.StatusOK, suggestions)
}
