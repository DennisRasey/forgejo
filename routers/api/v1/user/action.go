// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"net/http"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	secret_model "forgejo.org/models/secret"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/util"
	"forgejo.org/modules/web"
	"forgejo.org/routers/api/v1/utils"
	actions_service "forgejo.org/services/actions"
	"forgejo.org/services/context"
	secrets_service "forgejo.org/services/secrets"
)

// ListActionsSecrets lists actions secrets of the current user
func (Action) ListActionsSecrets(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/secrets user userListActionsSecrets
	// ---
	// summary: List actions secrets of the current user
	// produces:
	// - application/json
	// parameters:
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
	//     "$ref": "#/responses/SecretList"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := &secret_model.FindSecretsOptions{
		OwnerID:     ctx.Doer().ID,
		ListOptions: utils.GetListOptions(ctx),
	}

	secrets, count, err := db.FindAndCount[secret_model.Secret](ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	apiSecrets := make([]*api.Secret, len(secrets))
	for k, v := range secrets {
		apiSecrets[k] = &api.Secret{
			Name:    v.Name,
			Created: v.CreatedUnix.AsTime(),
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, apiSecrets)
}

// create or update one secret of the user scope
func (Action) CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /user/actions/secrets/{secretname} user updateUserSecret
	// ---
	// summary: Create or Update a secret value in a user scope
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateOrUpdateSecretOption"
	// responses:
	//   "201":
	//     description: response when creating a secret
	//   "204":
	//     description: response when updating a secret
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)

	_, created, err := secrets_service.CreateOrUpdateSecret(ctx, ctx.Doer().ID, 0, ctx.Params("secretname"), opt.Data)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "CreateOrUpdateSecret", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "CreateOrUpdateSecret", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateOrUpdateSecret", err)
		}
		return
	}

	if created {
		ctx.Status(http.StatusCreated)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// DeleteSecret delete one secret of the user scope
func (Action) DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /user/actions/secrets/{secretname} user deleteUserSecret
	// ---
	// summary: Delete a secret in a user scope
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: delete one secret of the user
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := secrets_service.DeleteSecretByName(ctx, ctx.Doer().ID, 0, ctx.Params("secretname"))
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "DeleteSecret", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DeleteSecret", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteSecret", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateVariable create a user-level variable
func (Action) CreateVariable(ctx *context.APIContext) {
	// swagger:operation POST /user/actions/variables/{variablename} user createUserVariable
	// ---
	// summary: Create a user-level variable
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateVariableOption"
	// responses:
	//   "201":
	//     description: response when creating a variable
	//   "204":
	//     description: response when creating a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateVariableOption)

	ownerID := ctx.Doer().ID
	variableName := ctx.Params("variablename")

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID: ownerID,
		Name:    variableName,
	})
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.Error(http.StatusInternalServerError, "GetVariable", err)
		return
	}
	if v != nil && v.ID > 0 {
		ctx.Error(http.StatusConflict, "VariableNameAlreadyExists", util.NewAlreadyExistErrorf("variable name %s already exists", variableName))
		return
	}

	if _, err := actions_service.CreateVariable(ctx, ownerID, 0, variableName, opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "CreateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateVariable", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UpdateVariable update a user-level variable which is created by current doer
func (Action) UpdateVariable(ctx *context.APIContext) {
	// swagger:operation PUT /user/actions/variables/{variablename} user updateUserVariable
	// ---
	// summary: Update a user-level variable which is created by current doer
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UpdateVariableOption"
	// responses:
	//   "201":
	//     description: response when updating a variable
	//   "204":
	//     description: response when updating a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.UpdateVariableOption)

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID: ctx.Doer().ID,
		Name:    ctx.Params("variablename"),
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetVariable", err)
		}
		return
	}

	if opt.Name == "" {
		opt.Name = ctx.Params("variablename")
	}
	if _, err := actions_service.UpdateVariable(ctx, v.ID, ctx.Doer().ID, 0, opt.Name, opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "UpdateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateVariable", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteVariable delete a user-level variable which is created by current doer
func (Action) DeleteVariable(ctx *context.APIContext) {
	// swagger:operation DELETE /user/actions/variables/{variablename} user deleteUserVariable
	// ---
	// summary: Delete a user-level variable which is created by current doer
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     description: response when deleting a variable
	//   "204":
	//     description: response when deleting a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := actions_service.DeleteVariableByName(ctx, ctx.Doer().ID, 0, ctx.Params("variablename")); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "DeleteVariableByName", err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DeleteVariableByName", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteVariableByName", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// GetVariable get a user-level variable which is created by current doer
func (Action) GetVariable(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/variables/{variablename} user getUserVariable
	// ---
	// summary: Get a user-level variable which is created by current doer
	// produces:
	// - application/json
	// parameters:
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//			"$ref": "#/responses/ActionVariable"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		OwnerID: ctx.Doer().ID,
		Name:    ctx.Params("variablename"),
	})
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetVariable", err)
		}
		return
	}

	variable := &api.ActionVariable{
		OwnerID: v.OwnerID,
		RepoID:  v.RepoID,
		Name:    v.Name,
		Data:    v.Data,
	}

	ctx.JSON(http.StatusOK, variable)
}

// ListVariables list user-level variables
func (Action) ListVariables(ctx *context.APIContext) {
	// swagger:operation GET /user/actions/variables user getUserVariablesList
	// ---
	// summary: Get the user-level list of variables which is created by current doer
	// produces:
	// - application/json
	// parameters:
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
	//			"$ref": "#/responses/VariableList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	vars, count, err := db.FindAndCount[actions_model.ActionVariable](ctx, &actions_model.FindVariablesOpts{
		OwnerID:     ctx.Doer().ID,
		ListOptions: utils.GetListOptions(ctx),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindVariables", err)
		return
	}

	variables := make([]*api.ActionVariable, len(vars))
	for i, v := range vars {
		variables[i] = &api.ActionVariable{
			OwnerID: v.OwnerID,
			RepoID:  v.RepoID,
			Name:    v.Name,
			Data:    v.Data,
		}
	}

	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, variables)
}

var _ actions_service.API = new(Action)

// Action implements actions_service.API
type Action struct{}

// NewAction creates a new Action service
func NewAction() actions_service.API {
	return Action{}
}
