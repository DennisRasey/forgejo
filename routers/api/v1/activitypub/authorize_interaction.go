// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"net/http"
	"strconv"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
)

// Fetch and load a remote object
func AuthorizeInteraction(ctx *context.Context) {
	resp, err := activitypub.Fetch(ctx.Req.URL.Query().Get("uri"))
	if err != nil {
		ctx.ServerError("Fetch", err)
		return
	}

	ap.ItemTyperFunc = forgefed.GetItemByType
	ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
	ap.IsNotEmpty = forgefed.NotEmpty
	object, err := ap.UnmarshalJSON(resp)
	if err != nil {
		ctx.ServerError("UnmarshalJSON", err)
		return
	}

	switch object.GetType() {
	case ap.PersonType:
		// Federated user
		person, err := ap.ToActor(object)
		if err != nil {
			ctx.ServerError("ToActor", err)
			return
		}
		err = createPerson(ctx, person)
		if err != nil {
			ctx.ServerError("CreatePerson", err)
			return
		}
		user, err := user_model.GetUserByIRI(ctx, object.GetLink().String())
		if err != nil {
			ctx.ServerError("GetUserByIRI", err)
			return
		}
		ctx.Redirect(setting.AppURL + user.Name)
	case forgefed.RepositoryType:
		// Federated repository
		err = forgefed.OnRepository(object, func(r *forgefed.Repository) error {
			return createRepository(ctx, r)
		})
		if err != nil {
			ctx.ServerError("CreateRepository", err)
			return
		}
		repo, err := repo_model.GetRepositoryByIRI(ctx, object.GetLink().String())
		if err != nil {
			ctx.ServerError("RepositoryIRIToName", err)
			return
		}
		ctx.Redirect(setting.AppURL + repo.OwnerName + "/" + repo.Name)
	case forgefed.TicketType:
		// Federated issue or pull request
		err = forgefed.OnTicket(object, func(t *forgefed.Ticket) error {
			return createTicket(ctx, t)
		})
		if err != nil {
			ctx.ServerError("ReceiveIssue", err)
			return
		}
		username, reponame, idx, err := activitypub.TicketIRIToName(object.GetLink())
		if err != nil {
			ctx.ServerError("TicketIRIToName", err)
			return
		}
		ctx.Redirect(setting.AppURL + username + "/" + reponame + "/issues/" + strconv.FormatInt(idx, 10))
	default:
		ctx.ServerError("Not implemented", err)
		return
	}

	ctx.Status(http.StatusOK)
}
