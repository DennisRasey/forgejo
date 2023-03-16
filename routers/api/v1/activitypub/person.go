// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
)

// Person function returns the Person actor for a user
func Person(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user-id/{user-id} activitypub activitypubPerson
	// ---
	// summary: Returns the Person actor for a user
	// produces:
	// - application/activity+json
	// parameters:
	// - name: user-id
	//   in: path
	//   description: user ID of the user
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	// TODO: the setting.AppURL during the test doesn't follow the definition: "It always has a '/' suffix"
	iri := fmt.Sprintf("%s/api/v1/activitypub/user-id/%d", strings.TrimSuffix(setting.AppURL, "/"), ctx.ContextUser.ID)
	person := ap.PersonNew(ap.IRI(iri))

	person.Name = ap.NaturalLanguageValuesNew()
	err := person.Name.Set("en", ap.Content(ctx.ContextUser.FullName))
	if err != nil {
		ctx.ServerError("Set Name", err)
		return
	}

	person.PreferredUsername = ap.NaturalLanguageValuesNew()
	err = person.PreferredUsername.Set("en", ap.Content(ctx.ContextUser.Name))
	if err != nil {
		ctx.ServerError("Set PreferredUsername", err)
		return
	}

	person.URL = ap.IRI(ctx.ContextUser.HTMLURL())
	person.Location = ap.IRI(ctx.ContextUser.GetEmail())

	person.Icon = ap.Image{
		Type:      ap.ImageType,
		MediaType: "image/png",
		URL:       ap.IRI(ctx.ContextUser.AvatarFullLinkWithSize(ctx, 2048)),
	}

	person.Inbox = ap.IRI(iri + "/inbox")
	person.Outbox = ap.IRI(iri + "/outbox")
	person.Following = ap.IRI(iri + "/following")
	person.Followers = ap.IRI(iri + "/followers")
	person.Liked = ap.IRI(iri + "/liked")

	person.PublicKey.ID = ap.IRI(iri + "#main-key")
	person.PublicKey.Owner = ap.IRI(iri)
	publicKeyPem, err := activitypub.GetPublicKey(ctx, ctx.ContextUser)
	if err != nil {
		ctx.ServerError("GetPublicKey", err)
		return
	}
	person.PublicKey.PublicKeyPem = publicKeyPem

	response(ctx, person)
}

// PersonInbox function handles the incoming data for a user inbox
func PersonInbox(ctx *context.APIContext) {
	// swagger:operation POST /activitypub/user-id/{user-id}/inbox activitypub activitypubPersonInbox
	// ---
	// summary: Send to the inbox
	// produces:
	// - application/activity+json
	// parameters:
	// - name: user-id
	//   in: path
	//   description: user ID of the user
	//   type: integer
	//   required: true
	// responses:
	//   "202":
	//     "$ref": "#/responses/empty"

	body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, setting.Federation.MaxSize))
	if err != nil {
		ctx.ServerError("Error reading request body", err)
		return
	}

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
	case ap.FollowType:
		// Following a user
		err = follow(ctx, activity)
	case ap.UndoType:
		// Unfollowing a user
		err = unfollow(ctx, activity)
	case ap.CreateType:
		if activity.Object.GetType() == ap.NoteType {
			// TODO: this is kinda a hack
			err = ap.OnObject(activity.Object, func(n *ap.Note) error {
				noteIRI := n.InReplyTo.GetLink().String()
				noteIRISplit := strings.Split(noteIRI, "/")
				n.Context = ap.IRI(strings.TrimSuffix(noteIRI, "/"+noteIRISplit[len(noteIRISplit)-1]))
				return createComment(ctx, n)
			})
		}
	case ap.DeleteType:
		// Deleting a user
		err = delete(ctx, activity)
	default:
		err = fmt.Errorf("unsupported ActivityStreams activity type: %s", activity.GetType())
	}
	if err != nil {
		ctx.ServerError("Could not process activity", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// PersonOutbox function returns the user's Outbox OrderedCollection
func PersonOutbox(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/outbox activitypub activitypubPersonOutbox
	// ---
	// summary: Returns the Outbox OrderedCollection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "501":
	//     "$ref": "#/responses/empty"

	ctx.Status(http.StatusNotImplemented)
}

// PersonFollowing function returns the user's Following Collection
func PersonFollowing(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/following activitypub activitypubPersonFollowing
	// ---
	// summary: Returns the Following Collection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	listOptions := utils.GetListOptions(ctx)
	users, count, err := user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, listOptions)
	if err != nil {
		ctx.ServerError("GetUserFollowing", err)
		return
	}
	items := make([]string, 0)
	for _, user := range users {
		items = append(items, user.GetIRI())
	}
	responseCollection(ctx, ctx.ContextUser.GetIRI()+"/following", listOptions, items, count)
}

// PersonFollowers function returns the user's Followers Collection
func PersonFollowers(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/followers activitypub activitypubPersonFollowers
	// ---
	// summary: Returns the Followers Collection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	listOptions := utils.GetListOptions(ctx)
	users, count, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, listOptions)
	if err != nil {
		ctx.ServerError("GetUserFollowers", err)
		return
	}
	items := make([]string, 0)
	for _, user := range users {
		items = append(items, user.GetIRI())
	}
	responseCollection(ctx, ctx.ContextUser.GetIRI()+"/followers", listOptions, items, count)
}

// PersonLiked function returns the user's Liked Collection
func PersonLiked(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/followers activitypub activitypubPersonLiked
	// ---
	// summary: Returns the Liked Collection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	listOptions := utils.GetListOptions(ctx)
	repos, count, err := repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
		ListOptions: listOptions,
		Actor:       ctx.Doer,
		Private:     ctx.IsSigned,
		StarredByID: ctx.ContextUser.ID,
	})
	if err != nil {
		ctx.ServerError("GetUserStarred", err)
		return
	}
	items := make([]string, 0)
	for _, repo := range repos {
		items = append(items, repo.GetIRI())
	}
	responseCollection(ctx, ctx.ContextUser.GetIRI()+"/liked", listOptions, items, count)
}
