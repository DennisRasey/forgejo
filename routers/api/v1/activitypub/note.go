// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/activitypub"
)

// Note function returns the Note object for a comment to an issue or PR
func Note(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/note/{username}/{reponame}/{noteid} activitypub activitypubNote
	// ---
	// summary: Returns the Note object for a comment to an issue or PR
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
	// - name: noteid
	//   in: path
	//   description: ID number of the comment
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	comment, err := issues_model.GetCommentByID(ctx, ctx.ParamsInt64("noteid"))
	if err != nil {
		if issues_model.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	// Ensure the comment comes from the specified repository.
	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	// Only allow comments and not events.
	if comment.Type != issues_model.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	note, err := activitypub.Note(ctx, comment)
	if err != nil {
		ctx.ServerError("Note", err)
		return
	}
	response(ctx, note)
}
