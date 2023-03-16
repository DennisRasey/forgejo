// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/activitypub"
)

// Ticket function returns the Ticket object for an issue or PR
func Ticket(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/ticket/{username}/{reponame}/{id} activitypub forgefedTicket
	// ---
	// summary: Returns the Ticket object for an issue or PR
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: reponame
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: ID number of the issue or PR
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.ParamsInt64("id"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}

	ticket, err := activitypub.Ticket(ctx, issue)
	if err != nil {
		ctx.ServerError("Ticket", err)
		return
	}
	response(ctx, ticket)
}
