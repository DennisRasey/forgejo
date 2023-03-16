// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/activitypub"
)

// https://datatracker.ietf.org/doc/html/draft-ietf-appsawg-webfinger-14#section-4.4

// WebfingerQuery returns information about a resource
// https://datatracker.ietf.org/doc/html/rfc7565
func WebfingerQuery(ctx *context.Context) {
	appURL, _ := url.Parse(setting.AppURL)

	resource, err := url.Parse(ctx.FormTrim("resource"))
	if err != nil {
		ctx.Error(http.StatusBadRequest)
		return
	}

	var u *user_model.User

	switch resource.Scheme {
	case "acct":
		// allow only the current host
		parts := strings.SplitN(resource.Opaque, "@", 2)
		if len(parts) != 2 {
			ctx.Error(http.StatusBadRequest)
			return
		}
		if parts[1] != appURL.Host {
			ctx.Error(http.StatusBadRequest)
			return
		}

		u, err = user_model.GetUserByName(ctx, parts[0])
	case "mailto":
		u, err = user_model.GetUserByEmail(ctx, resource.Opaque)
		if u != nil && u.KeepEmailPrivate {
			err = user_model.ErrUserNotExist{}
		}
	case "https":
		u, err = user_model.GetUserByIRI(ctx, ctx.FormString("resource"))
	default:
		ctx.Error(http.StatusBadRequest)
		return
	}
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Error(http.StatusNotFound)
		} else {
			log.Error("Error getting user: %s Error: %v", resource.Opaque, err)
			ctx.Error(http.StatusInternalServerError)
		}
		return
	}

	if !user_model.IsUserVisibleToViewer(ctx, u, ctx.Doer) {
		ctx.Error(http.StatusNotFound)
		return
	}

	aliases := []string{
		u.HTMLURL(),
		appURL.String() + "api/v1/activitypub/user-id/" + fmt.Sprint(u.ID),
	}
	if !u.KeepEmailPrivate {
		aliases = append(aliases, fmt.Sprintf("mailto:%s", u.Email))
	}

	links := []*activitypub.WebfingerLink{
		{
			Rel:  "http://webfinger.net/rel/profile-page",
			Type: "text/html",
			Href: u.HTMLURL(),
		},
		{
			Rel:  "http://webfinger.net/rel/avatar",
			Href: u.AvatarLink(ctx),
		},
		{
			Rel:  "self",
			Type: "application/activity+json",
			Href: appURL.String() + "api/v1/activitypub/user-id/" + fmt.Sprint(u.ID),
		},
		{
			Rel:  "http://openid.net/specs/connect/1.0/issuer",
			Href: appURL.String(),
		},
		{
			Rel:      "http://ostatus.org/schema/1.0/subscribe",
			Template: appURL.String() + "api/v1/authorize_interaction?uri={uri}",
		},
	}

	ctx.Resp.Header().Add("Access-Control-Allow-Origin", "*")
	ctx.JSON(http.StatusOK, &activitypub.WebfingerJRD{
		Subject: fmt.Sprintf("acct:%s@%s", url.QueryEscape(u.Name), appURL.Host),
		Aliases: aliases,
		Links:   links,
	})
}
