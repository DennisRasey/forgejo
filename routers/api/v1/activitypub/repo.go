// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

// Repo function returns the Repository actor of a repo
func Repo(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/{reponame} activitypub activitypubRepo
	// ---
	// summary: Returns the repository
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
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	iri := ctx.Repo.Repository.GetIRI()
	repo := forgefed.RepositoryNew(ap.IRI(iri))

	repo.Name = ap.NaturalLanguageValuesNew()
	err := repo.Name.Set("en", ap.Content(ctx.Repo.Repository.Name))
	if err != nil {
		ctx.ServerError("Set Name", err)
		return
	}

	repo.AttributedTo = ap.IRI(ctx.Repo.Owner.GetIRI())

	repo.Summary = ap.NaturalLanguageValuesNew()
	err = repo.Summary.Set("en", ap.Content(ctx.Repo.Repository.Description))
	if err != nil {
		ctx.ServerError("Set Description", err)
		return
	}

	repo.Inbox = ap.IRI(iri + "/inbox")
	repo.Outbox = ap.IRI(iri + "/outbox")
	repo.Followers = ap.IRI(iri + "/followers")
	repo.Team = ap.IRI(iri + "/team")

	response(ctx, repo)
}

// RepoInbox function handles the incoming data for a repo inbox
func RepoInbox(ctx *context.APIContext) {
	// swagger:operation POST /activitypub/repo/{username}/{reponame}/inbox activitypub activitypubRepoInbox
	// ---
	// summary: Send to the inbox
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
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "202":
	//     "$ref": "#/responses/empty"

	body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, setting.Federation.MaxSize))
	if err != nil {
		ctx.ServerError("Error reading request body", err)
		return
	}

	ap.ItemTyperFunc = forgefed.GetItemByType
	ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
	ap.IsNotEmpty = forgefed.NotEmpty
	var activity ap.Activity
	err = activity.UnmarshalJSON(body)
	if err != nil {
		ctx.ServerError("UnmarshalJSON", err)
		return
	}

	// Make sure keyID matches the user doing the activity
	_, keyID, _ := getKeyID(ctx.Req)
	err = checkActivityAndKeyID(activity, keyID)
	if err != nil {
		ctx.ServerError("keyID does not match activity", err)
		return
	}

	// Process activity
	switch activity.Type {
	case ap.CreateType:
		switch activity.Object.GetType() {
		case forgefed.RepositoryType:
			// Fork created by remote instance
			err = forgefed.OnRepository(activity.Object, func(r *forgefed.Repository) error {
				return createRepository(ctx, r)
			})
		case forgefed.TicketType:
			// New issue or pull request
			err = forgefed.OnTicket(activity.Object, func(t *forgefed.Ticket) error {
				return createTicket(ctx, t)
			})
		case ap.NoteType:
			// New comment
			err = ap.On(activity.Object, func(n *ap.Note) error {
				return createComment(ctx, n)
			})
		default:
			err = fmt.Errorf("unsupported ActivityStreams object type: %s", activity.Object.GetType())
		}
	case ap.LikeType:
		// Starring a repo
		err = star(ctx, activity)
	case ap.UndoType:
		// Unstarring a repo
		err = unstar(ctx, activity)
	default:
		err = fmt.Errorf("unsupported ActivityStreams activity type: %s", activity.GetType())
	}
	if err != nil {
		ctx.ServerError("Could not process activity", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// RepoOutbox function returns the repo's Outbox OrderedCollection
func RepoOutbox(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/{reponame}/outbox activitypub activitypubRepoOutbox
	// ---
	// summary: Returns the outbox
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
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "501":
	//     "$ref": "#/responses/empty"

	ctx.Status(http.StatusNotImplemented)
}

// RepoFollowers function returns the repo's Followers OrderedCollection
func RepoFollowers(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/{reponame}/followers activitypub activitypubRepoFollowers
	// ---
	// summary: Returns the followers collection
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
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	// TODO
	ctx.Status(http.StatusNotImplemented)
}
