// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"fmt"
	gotemplate "html/template"
	"io"
	"net/http"
	"path"
	"strings"

	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/base"
	"forgejo.org/modules/git"
	"forgejo.org/modules/log"
	"forgejo.org/modules/markup"
	"forgejo.org/modules/markup/markdown"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
	shared_user "forgejo.org/routers/web/shared/user"
	"forgejo.org/services/context"
	funding_service "forgejo.org/services/funding"
)

const (
	tplOrgHome base.TplName = "org/home"
)

// Home show organization home page
func Home(ctx *context.Context) {
	uname := ctx.Params(":username")

	if strings.HasSuffix(uname, ".keys") || strings.HasSuffix(uname, ".gpg") {
		ctx.NotFound("", nil)
		return
	}

	ctx.SetParams(":org", uname)
	context.HandleOrgAssignment(ctx)
	if ctx.Written() {
		return
	}

	org := ctx.Org.Organization

	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Title"] = org.DisplayName()

	ctx.Data["OpenGraphTitle"] = ctx.ContextUser.DisplayName()
	ctx.Data["OpenGraphType"] = "profile"
	ctx.Data["OpenGraphImageURL"] = ctx.ContextUser.AvatarLink(ctx)
	ctx.Data["OpenGraphURL"] = ctx.ContextUser.HTMLURL()
	ctx.Data["OpenGraphDescription"] = ctx.ContextUser.Description

	var orderBy db.SearchOrderBy
	sortOrder := ctx.FormString("sort")
	if _, ok := repo_model.OrderByFlatMap[sortOrder]; !ok {
		sortOrder = setting.UI.ExploreDefaultSort // TODO: add new default sort order for org home?
	}
	ctx.Data["SortType"] = sortOrder
	orderBy = repo_model.OrderByFlatMap[sortOrder]

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	archived := ctx.FormOptionalBool("archived")
	ctx.Data["IsArchived"] = archived

	fork := ctx.FormOptionalBool("fork")
	ctx.Data["IsFork"] = fork

	mirror := ctx.FormOptionalBool("mirror")
	ctx.Data["IsMirror"] = mirror

	template := ctx.FormOptionalBool("template")
	ctx.Data["IsTemplate"] = template

	private := ctx.FormOptionalBool("private")
	ctx.Data["IsPrivate"] = private

	var (
		repos []*repo_model.Repository
		count int64
		err   error
	)
	repos, count, err = repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Keyword:            keyword,
		OwnerID:            org.ID,
		Collaborate:        optional.Some(false), // A organisation doesn't collaborate to any repository, avoid doing expensive SQL query.
		OrderBy:            orderBy,
		Private:            ctx.IsSigned,
		Actor:              ctx.Doer,
		Language:           language,
		IncludeDescription: setting.UI.SearchRepoDescription,
		Archived:           archived,
		Fork:               fork,
		Mirror:             mirror,
		Template:           template,
		IsPrivate:          private,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	opts := &organization.FindOrgMembersOpts{
		Doer:         ctx.Doer,
		OrgID:        org.ID,
		IsDoerMember: ctx.Org.IsMember,
		ListOptions:  db.ListOptions{Page: 1, PageSize: 25},
	}

	members, _, err := organization.FindOrgMembers(ctx, opts)
	if err != nil {
		ctx.ServerError("FindOrgMembers", err)
		return
	}

	if ctx.Doer != nil {
		invite, err := organization.GetInviteByOrgAndUser(ctx, org.ID, ctx.Doer.ID)
		if err != nil && !organization.IsErrTeamInviteNotFound(err) {
			ctx.ServerError("GetInviteByOrgAndUser", err)
			return
		}
		ctx.Data["Invite"] = invite
		if invite != nil {
			teams, err := organization.GetTeamsInvitedTo(ctx, org.ID, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("GetTeamsInvitedTo", err)
				return
			}
			ctx.Data["TeamsInvitedTo"] = teams
		}
	}

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count
	ctx.Data["Members"] = members
	ctx.Data["Teams"] = ctx.Org.Teams
	ctx.Data["DisableNewPullMirrors"] = setting.Mirror.DisableNewPull
	ctx.Data["PageIsViewRepositories"] = true

	err = shared_user.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	pager := context.NewPagination(int(count), setting.UI.User.RepoPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	pager.AddParamString("language", language)
	if has, value := archived.Get(); has {
		pager.AddParamString("archived", fmt.Sprint(value))
	}
	if has, value := fork.Get(); has {
		pager.AddParamString("fork", fmt.Sprint(value))
	}
	if has, value := mirror.Get(); has {
		pager.AddParamString("mirror", fmt.Sprint(value))
	}
	if has, value := template.Get(); has {
		pager.AddParamString("template", fmt.Sprint(value))
	}
	if has, value := private.Get(); has {
		pager.AddParamString("private", fmt.Sprint(value))
	}
	ctx.Data["Page"] = pager

	ctx.Data["ShowMemberAndTeamTab"] = ctx.Org.IsMember || len(members) > 0

	profileDbRepo, profileGitRepo, profileReadmeBlob, profileFunding, profileClose := shared_user.FindUserProfileReadme(ctx, ctx.Doer)
	defer profileClose()
	prepareOrgProfileReadme(ctx, profileGitRepo, profileDbRepo, profileReadmeBlob)
	prepareOrgProfileFunding(ctx, profileFunding)

	ctx.HTML(http.StatusOK, tplOrgHome)
}

func prepareOrgProfileReadme(ctx *context.Context, profileGitRepo *git.Repository, profileDbRepo *repo_model.Repository, profileReadme *git.Blob) {
	if profileGitRepo == nil || profileReadme == nil {
		return
	}

	if rc, _, err := profileReadme.NewTruncatedReader(setting.UI.MaxDisplayFileSize); err != nil {
		log.Error("failed to NewTruncatedReader: %v", err)
	} else {
		defer rc.Close()

		if markupType := markup.Type(profileReadme.Name()); markupType != "" {
			if profileContent, err := markdown.RenderReader(&markup.RenderContext{
				Ctx:     ctx,
				Type:    markupType,
				GitRepo: profileGitRepo,
				Links: markup.Links{
					// Pass repo link to markdown render for the full link of media elements.
					// The profile of default branch would be shown.
					Base:       profileDbRepo.Link(),
					BranchPath: path.Join("branch", util.PathEscapeSegments(profileDbRepo.DefaultBranch)),
				},
				Metas: map[string]string{"mode": "document"},
			}, rc); err != nil {
				log.Error("failed to RenderString: %v", err)
			} else {
				ctx.Data["ProfileReadme"] = profileContent
			}
		} else {
			content, err := io.ReadAll(rc)
			if err != nil {
				log.Error("Read readme content failed: %v", err)
			}
			ctx.Data["ProfileReadme"] = gotemplate.HTMLEscapeString(util.UnsafeBytesToString(content))
			ctx.Data["IsProfileReadmePlain"] = true
		}
	}
}

func prepareOrgProfileFunding(ctx *context.Context, profileFunding *funding_service.RepoFunding) {
	if profileFunding == nil || len(profileFunding.Entries) == 0 {
		return
	}

	ctx.Data["Funding"] = profileFunding.Entries
	ctx.Data["FundingConfig"] = profileFunding.ConfigPath
	ctx.Data["FundingHasErrors"] = len(profileFunding.Errors) > 0
	ctx.Data["FundingTarget"] = ctx.ContextUser.DisplayName()
}
