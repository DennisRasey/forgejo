// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"forgejo.org/modules/markup"
	"forgejo.org/modules/markup/markdown"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
	"forgejo.org/services/context"

	"mvdan.cc/xurls/v2"
)

type Renderer struct {
	Mode, Text, URLPrefix, FilePath, BranchPath string
	IsWiki                                      bool
}

type RenderParams struct {
	markupType, relativePath string
	meta                     map[string]string
}

func (re *Renderer) prepare(ctx *context.Base, repo *context.Repository) (RenderParams, error) {
	params := RenderParams{markupType: "", relativePath: "", meta: map[string]string{}}

	if len(re.Text) == 0 {
		_, _ = ctx.Write([]byte(""))
		return params, errors.New("re.Text empty")
	}

	switch re.Mode {
	case "markdown":
		// Raw markdown
		if err := markdown.RenderRaw(&markup.RenderContext{
			Ctx: ctx,
			Links: markup.Links{
				AbsolutePrefix: true,
				Base:           re.URLPrefix,
			},
		}, strings.NewReader(re.Text), ctx.Resp); err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return params, errors.New("RenderRaw")
	case "comment":
		// Comment as markdown
		params.markupType = markdown.MarkupName
	case "gfm":
		// Github Flavored Markdown as document
		params.markupType = markdown.MarkupName
	case "file":
		// File as document based on file extension
		params.markupType = ""
		params.relativePath = re.FilePath
	default:
		ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Unknown mode: %s", re.Mode))
		return params, errors.New("re.Mode unknown " + re.Mode)
	}

	if !strings.HasPrefix(setting.AppSubURL+"/", re.URLPrefix) {
		// check if urlPrefix is already set to a URL
		linkRegex, _ := xurls.StrictMatchingScheme("https?://")
		m := linkRegex.FindStringIndex(re.URLPrefix)
		if m == nil {
			re.URLPrefix = util.URLJoin(setting.AppURL, re.URLPrefix)
		}
	}

	if repo != nil && repo.Repository != nil {
		if re.Mode == "comment" {
			params.meta = repo.Repository.ComposeMetas(ctx)
		} else {
			params.meta = repo.Repository.ComposeDocumentMetas(ctx)
		}
	}
	if re.Mode != "comment" {
		params.meta["mode"] = "document"
	}

	return params, nil
}

// RenderMarkup renders markup text for the /markup and /markdown endpoints
func (re *Renderer) RenderMarkup(ctx *context.Base, repo *context.Repository) {
	params, err := re.prepare(ctx, repo)
	if err != nil {
		return
	}

	if err := markup.Render(&markup.RenderContext{
		Ctx: ctx,
		Links: markup.Links{
			AbsolutePrefix: true,
			Base:           re.URLPrefix,
			BranchPath:     re.BranchPath,
		},
		Metas:        params.meta,
		IsWiki:       re.IsWiki,
		Type:         params.markupType,
		RelativePath: params.relativePath,
	}, strings.NewReader(re.Text), ctx.Resp); err != nil {
		if markup.IsErrUnsupportedRenderExtension(err) || markup.IsErrMissingExtension(err) {
			ctx.Error(http.StatusUnprocessableEntity, err.Error())
		} else {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}
}

func (re *Renderer) RenderMarkupWebContext(ctx *context.Context, repo *context.Repository) {
	params, err := re.prepare(ctx.Base, repo)
	if err != nil {
		return
	}

	if err := markup.Render(&markup.RenderContext{
		Ctx: ctx,
		Links: markup.Links{
			AbsolutePrefix: false,
			Base:           re.URLPrefix,
			BranchPath:     re.BranchPath,
		},
		Metas:        params.meta,
		IsWiki:       re.IsWiki,
		Type:         params.markupType,
		RelativePath: params.relativePath,
	}, strings.NewReader(re.Text), ctx.Resp); err != nil {
		if markup.IsErrUnsupportedRenderExtension(err) || markup.IsErrMissingExtension(err) {
			ctx.Error(http.StatusUnprocessableEntity, err.Error())
		} else {
			ctx.Error(http.StatusInternalServerError, err.Error())
		}
		return
	}
}
