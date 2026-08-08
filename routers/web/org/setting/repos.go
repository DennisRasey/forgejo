// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package setting

import (
	"net/http"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/base"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/services/context"
)

const tplSettingsRepositories base.TplName = "org/settings/repos"

// Repos renders a sortable, paginated table of the repositories owned by the
// organization; collaborations are excluded.
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.repos")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsOrgSettingsRepos"] = true

	sortOrder := ctx.FormString("sort")
	if _, ok := repo_model.OrderByFlatMap[sortOrder]; !ok {
		sortOrder = "recentupdate"
	}
	ctx.Data["SortType"] = sortOrder

	page := max(1, ctx.FormInt("page"))
	pageSize := setting.UI.Admin.RepoPagingNum

	repos, count, err := repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: pageSize,
		},
		Actor:       ctx.Doer,
		OwnerID:     ctx.Org.Organization.ID,
		OrderBy:     repo_model.OrderByFlatMap[sortOrder],
		Collaborate: optional.Some(false), // organizations cannot collaborate on repositories; restrict the query to owned ones
		Private:     true,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}
	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count

	pager := context.NewPagination(int(count), pageSize, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplSettingsRepositories)
}
