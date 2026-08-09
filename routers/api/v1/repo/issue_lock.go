// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "forgejo.org/models/issues"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/web"
	"forgejo.org/services/context"
)

// LockIssue locks an issue
func LockIssue(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/issues/{index}/lock issue lockIssue
	// ---
	// summary: Lock an issue
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue to lock
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueLockOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"
	issue := getIssueFromContext(ctx)

	if issue == nil {
		return
	}

	form := web.GetForm(ctx).(*api.IssueLockOption)

	if issue.IsLocked {
		// The issue is already locked. Even though this is functionally
		// a no-op, the locking reason provided could differ from the reason
		// provided when it was originally locked, so we return a 409 instead
		// (since the issue is locked, but the reason was not updated).
		ctx.Status(http.StatusConflict)
		return
	}

	// If we don't do this, it will crash when trying to add the lock event to the comment history
	err := issue.LoadRepo(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepo", err)
		return
	}

	if err := issues_model.LockIssue(ctx, &issues_model.IssueLockOptions{
		Doer:   ctx.Doer(),
		Issue:  issue,
		Reason: form.Reason,
	}); err != nil {
		ctx.ServerError("LockIssue", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UnlockIssue unlocks an issue
func UnlockIssue(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/lock issue unlockIssue
	// ---
	// summary: Unlock an issue
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: index
	//   in: path
	//   description: index of the issue to unlock
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"
	issue := getIssueFromContext(ctx)

	if issue == nil {
		return
	}

	if !issue.IsLocked {
		// Unlocking an issue that is already unlocked is a no-op,
		// but to remain consistent with the locking operation we return a
		// 409 here.
		ctx.Status(http.StatusConflict)
		return
	}

	// If we don't do this, it will crash when trying to add the unlock event to the comment history
	err := issue.LoadRepo(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepo", err)
		return
	}

	if err := issues_model.UnlockIssue(ctx, &issues_model.IssueLockOptions{
		Doer:  ctx.Doer(),
		Issue: issue,
	}); err != nil {
		ctx.ServerError("UnlockIssue", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
