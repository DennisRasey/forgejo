// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"forgejo.org/routers/api/v1/shared"
	"forgejo.org/services/context"
)

// GetRegistrationToken returns a token to register user-level runners
//
// Deprecated: This operation has been deprecated in Forgejo 15. Use the web UI or RegisterRunner instead.
func (Action) GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/runners/registration-token user userGetRunnerRegistrationToken
	// ---
	// summary: Get the user's runner registration token
	// description: >
	//   This operation has been deprecated in Forgejo 15.
	//   Use the web UI or [`/user/actions/runners`](#/user/registerUserRunner) instead.
	// deprecated: true
	// produces:
	// - application/json
	// parameters:
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	shared.GetRegistrationToken(ctx, ctx.Doer().ID, 0)
}

// SearchActionRunJobs returns a list of actions jobs filtered by the provided parameters
func (Action) SearchActionRunJobs(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/runners/jobs user userSearchRunJobs
	// ---
	// summary: Search for user's action jobs according filter conditions
	// produces:
	// - application/json
	// parameters:
	// - name: labels
	//   in: query
	//   description: a comma separated list of run job labels to search for
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunJobList"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	shared.GetActionRunJobs(ctx, ctx.Doer().ID, 0)
}

// ListRunners returns the user's runners
func (Action) ListRunners(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/runners user getUserRunners
	// ---
	// summary: Get the user's runners
	// produces:
	// - application/json
	// parameters:
	// - name: visible
	//   in: query
	//   description: whether to include all visible runners (true) or only those that are directly owned by the user (false)
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionRunnerList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.ListRunners(ctx, ctx.Doer().ID, 0)
}

// GetRunner gets a particular runner that belongs to the user
func (Action) GetRunner(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/runners/{runner_id} user getUserRunner
	// ---
	// summary: Get a particular runner that belongs to the user
	// produces:
	// - application/json
	// parameters:
	// - name: runner_id
	//   in: path
	//   description: ID of the runner
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionRunner"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.GetRunner(ctx, ctx.Doer().ID, 0, ctx.ParamsInt64("runner_id"))
}

// RegisterRunner registers a new user-level runner
func (Action) RegisterRunner(ctx *context.APIContext) {
	// swagger:operation POST /user/actions/runners user registerUserRunner
	// ---
	// summary: Register a new user-level runner
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/RegisterRunnerOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/RegisterRunnerResponse"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "404":
	//     "$ref": "#/responses/notFound"

	shared.RegisterRunner(ctx, ctx.Doer().ID, 0)
}

// DeleteRunner deletes a particular user-level runner
func (Action) DeleteRunner(ctx *context.APIContext) {
	// swagger:operation DELETE /user/actions/runners/{runner_id} user deleteUserRunner
	// ---
	// summary: Delete a particular user-level runner
	// produces:
	// - application/json
	// parameters:
	// - name: runner_id
	//   in: path
	//   description: ID of the runner
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: runner has been deleted
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.DeleteRunner(ctx, ctx.Doer().ID, 0, ctx.ParamsInt64("runner_id"))
}
