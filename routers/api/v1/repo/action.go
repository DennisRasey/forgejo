// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	secret_model "forgejo.org/models/secret"
	"forgejo.org/modules/actions"
	"forgejo.org/modules/log"
	"forgejo.org/modules/optional"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/util"
	"forgejo.org/modules/web"
	"forgejo.org/routers/api/v1/shared"
	"forgejo.org/routers/api/v1/utils"
	actions_service "forgejo.org/services/actions"
	"forgejo.org/services/context"
	"forgejo.org/services/convert"
	secrets_service "forgejo.org/services/secrets"
)

// ListActionsSecrets list an repo's actions secrets
func (Action) ListActionsSecrets(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/secrets repository repoListActionsSecrets
	// ---
	// summary: List an repo's actions secrets
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo().Repository

	opts := &secret_model.FindSecretsOptions{
		RepoID:      repo.ID,
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

// create or update one secret of the repository
func (Action) CreateOrUpdateSecret(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/secrets/{secretname} repository updateRepoSecret
	// ---
	// summary: Create or Update a secret value in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
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
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo().Repository

	opt := web.GetForm(ctx).(*api.CreateOrUpdateSecretOption)

	_, created, err := secrets_service.CreateOrUpdateSecret(ctx, 0, repo.ID, ctx.Params("secretname"), opt.Data)
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

// DeleteSecret delete one secret of the repository
func (Action) DeleteSecret(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/secrets/{secretname} repository deleteRepoSecret
	// ---
	// summary: Delete a secret in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: secretname
	//   in: path
	//   description: name of the secret
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: delete one secret of the organization
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo().Repository

	err := secrets_service.DeleteSecretByName(ctx, 0, repo.ID, ctx.Params("secretname"))
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

// GetVariable get a repo-level variable
func (Action) GetVariable(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/variables/{variablename} repository getRepoVariable
	// ---
	// summary: Get a repo-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: ctx.Repo().Repository.ID,
		Name:   ctx.Params("variablename"),
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

// DeleteVariable delete a repo-level variable
func (Action) DeleteVariable(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/variables/{variablename} repository deleteRepoVariable
	// ---
	// summary: Delete a repo-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: variablename
	//   in: path
	//   description: name of the variable
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: response when deleting a variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := actions_service.DeleteVariableByName(ctx, 0, ctx.Repo().Repository.ID, ctx.Params("variablename")); err != nil {
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

// CreateVariable create a repo-level variable
func (Action) CreateVariable(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/variables/{variablename} repository createRepoVariable
	// ---
	// summary: Create a repo-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
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
	//     description: response when creating a repo-level variable
	//   "204":
	//     description: response when creating a repo-level variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.CreateVariableOption)

	repoID := ctx.Repo().Repository.ID
	variableName := ctx.Params("variablename")

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: repoID,
		Name:   variableName,
	})
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		ctx.Error(http.StatusInternalServerError, "GetVariable", err)
		return
	}
	if v != nil && v.ID > 0 {
		ctx.Error(http.StatusConflict, "VariableNameAlreadyExists", util.NewAlreadyExistErrorf("variable name %s already exists", variableName))
		return
	}

	if _, err := actions_service.CreateVariable(ctx, 0, repoID, variableName, opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "CreateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateVariable", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// UpdateVariable update a repo-level variable
func (Action) UpdateVariable(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/actions/variables/{variablename} repository updateRepoVariable
	// ---
	// summary: Update a repo-level variable
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
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
	//     description: response when updating a repo-level variable
	//   "204":
	//     description: response when updating a repo-level variable
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.UpdateVariableOption)

	v, err := actions_service.GetVariable(ctx, actions_model.FindVariablesOpts{
		RepoID: ctx.Repo().Repository.ID,
		Name:   ctx.Params("variablename"),
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
	if _, err := actions_service.UpdateVariable(ctx, v.ID, 0, ctx.Repo().Repository.ID, opt.Name, opt.Value); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Error(http.StatusBadRequest, "UpdateVariable", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateVariable", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListVariables list repo-level variables
func (Action) ListVariables(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/variables repository getRepoVariablesList
	// ---
	// summary: Get repo-level variables list
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: name of the owner
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
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
	//		 "$ref": "#/responses/VariableList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	vars, count, err := db.FindAndCount[actions_model.ActionVariable](ctx, &actions_model.FindVariablesOpts{
		RepoID:      ctx.Repo().Repository.ID,
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

// GetRegistrationToken returns the runner registration token to register runners for the repository
//
// Deprecated: This operation has been deprecated in Forgejo 15. Use the web UI or RegisterRunner instead.
func (Action) GetRegistrationToken(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runners/registration-token repository repoGetRunnerRegistrationToken
	// ---
	// summary: Get a repository's runner registration token
	// description: >
	//   This operation has been deprecated in Forgejo 15.
	//   Use the web UI or [`/repos/{owner}/{repo}/actions/runners`](#/repository/registerRepoRunner) instead.
	// deprecated: true
	// produces:
	// - application/json
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/RegistrationToken"

	shared.GetRegistrationToken(ctx, 0, ctx.Repo().Repository.ID)
}

// ListRunners returns runners that belong to the repository
func (Action) ListRunners(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runners repository getRepoRunners
	// ---
	// summary: Get runners belonging to the repository
	// produces:
	// - application/json
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
	// - name: visible
	//   in: query
	//   description: whether to include all visible runners (true) or only those that are directly owned by the repository (false)
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.ListRunners(ctx, 0, ctx.Repo().Repository.ID)
}

// GetRunner returns a particular runner that belongs to the repository
func (Action) GetRunner(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runners/{runner_id} repository getRepoRunner
	// ---
	// summary: Get a particular runner that belongs to the repository
	// produces:
	// - application/json
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.GetRunner(ctx, 0, ctx.Repo().Repository.ID, ctx.ParamsInt64("runner_id"))
}

// RegisterRunner registers a new repository-level runner
func (Action) RegisterRunner(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runners repository registerRepoRunner
	// ---
	// summary: Register a new repository-level runner
	// consumes:
	// - application/json
	// produces:
	// - application/json
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

	shared.RegisterRunner(ctx, 0, ctx.Repo().Repository.ID)
}

// DeleteRunner removes a particular runner that belongs to a repository
func (Action) DeleteRunner(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/runners/{runner_id} repository deleteRepoRunner
	// ---
	// summary: Delete a particular runner that belongs to a repository
	// produces:
	// - application/json
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
	//   "404":
	//     "$ref": "#/responses/notFound"
	shared.DeleteRunner(ctx, 0, ctx.Repo().Repository.ID, ctx.ParamsInt64("runner_id"))
}

// SearchActionRunJobs return a list of actions jobs filtered by the provided parameters
func (Action) SearchActionRunJobs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runners/jobs repository repoSearchRunJobs
	// ---
	// summary: Search for repository's action jobs according filter conditions
	// produces:
	// - application/json
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
	// - name: labels
	//   in: query
	//   description: a comma separated list of run job labels to search for
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/RunJobList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	shared.GetActionRunJobs(ctx, 0, ctx.Repo().Repository.ID)
}

var _ actions_service.API = new(Action)

// Action implements actions_service.API
type Action struct{}

// NewAction creates a new Action service
func NewAction() actions_service.API {
	return Action{}
}

// ListActionTasks list all the actions of a repository
func ListActionTasks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/tasks repository ListActionTasks
	// ---
	// summary: List a repository's action tasks
	// produces:
	// - application/json
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// - name: status
	//   in: query
	//   description: |
	//     Returns workflow tasks with the check run status or conclusion that is specified.
	// 		 For example, a conclusion can be success or a status can be in_progress.
	//   type: array
	//   items:
	//     type: string
	//     enum: [unknown, waiting, running, success, failure, cancelled, skipped, blocked]
	// responses:
	//   "200":
	//     "$ref": "#/responses/TasksList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"
	//   "422":
	//     "$ref": "#/responses/validationError"

	statusStrs := ctx.FormStrings("status")
	statuses := make([]actions_model.Status, len(statusStrs))
	for i, s := range statusStrs {
		if status, exists := actions_model.StatusFromString(s); exists {
			statuses[i] = status
		} else {
			ctx.Error(http.StatusBadRequest, "StatusFromString", fmt.Sprintf("unknown status: %s", s))
			return
		}
	}

	tasks, total, err := db.FindAndCount[actions_model.ActionTask](ctx, &actions_model.FindTaskOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo().Repository.ID,
		Status:      statuses,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListActionTasks", err)
		return
	}

	res := new(api.ActionTaskResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionTask, len(tasks))
	for i := range tasks {
		convertedTask, err := convert.ToActionTask(ctx, tasks[i])
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ToActionTask", err)
			return
		}
		res.Entries[i] = convertedTask
	}

	ctx.JSON(http.StatusOK, &res)
}

// DispatchWorkflow dispatches a workflow
func DispatchWorkflow(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/workflows/{workflowfilename}/dispatches repository DispatchWorkflow
	// ---
	// summary: Dispatches a workflow
	// consumes:
	// - application/json
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
	// - name: workflowfilename
	//   in: path
	//   description: name of the workflow
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/DispatchWorkflowOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/DispatchWorkflowRun"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opt := web.GetForm(ctx).(*api.DispatchWorkflowOption)
	name := ctx.Params("workflowfilename")

	if len(opt.Ref) == 0 {
		ctx.Error(http.StatusBadRequest, "ref", "ref is empty")
		return
	} else if len(name) == 0 {
		ctx.Error(http.StatusBadRequest, "workflowfilename", "workflow file name is empty")
		return
	}

	workflow, err := actions_service.GetWorkflowFromCommit(ctx.Repo().GitRepo, opt.Ref, name)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetWorkflowFromCommit", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetWorkflowFromCommit", err)
		}
		return
	}

	inputGetter := func(key string) string {
		return opt.Inputs[key]
	}

	run, jobs, err := workflow.Dispatch(ctx, inputGetter, ctx.Repo().Repository, ctx.Doer())
	if err != nil {
		if actions_service.IsInputRequiredErr(err) {
			ctx.Error(http.StatusBadRequest, "workflow.Dispatch", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "workflow.Dispatch", err)
		}
		return
	}

	workflowRun := &api.DispatchWorkflowRun{
		ID:        run.ID,
		RunNumber: run.Index,
		Jobs:      jobs,
	}

	if opt.ReturnRunInfo {
		ctx.JSON(http.StatusCreated, workflowRun)
	} else {
		ctx.Status(http.StatusNoContent)
	}
}

// ListActionRuns return a filtered list of ActionRun
func ListActionRuns(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs repository ListActionRuns
	// ---
	// summary: List a repository's action runs
	// produces:
	// - application/json
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// - name: event
	//   in: query
	//   description: Returns workflow run triggered by the specified events. For example, `push`, `pull_request` or `workflow_dispatch`.
	//   type: array
	//   items:
	//     type: string
	// - name: status
	//   in: query
	//   description: |
	//     Returns workflow runs with the check run status or conclusion that is specified. For example, a conclusion can be success or a status can be in_progress. Only Forgejo Actions can set a status of waiting, pending, or requested.
	//   type: array
	//   items:
	//     type: string
	//     enum: [unknown, waiting, running, success, failure, cancelled, skipped, blocked]
	// - name: run_number
	//   in: query
	//   description: |
	//     Returns the workflow run associated with the run number.
	//   type: integer
	//   format: int64
	// - name: head_sha
	//   in: query
	//   description: Only returns workflow runs that are associated with the specified head_sha.
	//   type: string
	// - name: ref
	//   in: query
	//   description: Only return workflow runs that involve the given Git reference, for example, `refs/heads/main`.
	//   type: string
	// - name: workflow_id
	//   in: query
	//   description: Only return workflow runs that involve the given workflow ID.
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionRunList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	statusStrs := ctx.FormStrings("status")
	statuses := make([]actions_model.Status, len(statusStrs))
	for i, s := range statusStrs {
		if status, exists := actions_model.StatusFromString(s); exists {
			statuses[i] = status
		} else {
			ctx.Error(http.StatusBadRequest, "StatusFromString", fmt.Sprintf("unknown status: %s", s))
			return
		}
	}

	runs, total, err := db.FindAndCount[actions_model.ActionRun](ctx, &actions_model.FindRunOptions{
		ListOptions: utils.GetListOptions(ctx),
		OwnerID:     ctx.Repo().Owner.ID,
		RepoID:      ctx.Repo().Repository.ID,
		Events:      ctx.FormStrings("event"),
		Status:      statuses,
		RunNumber:   ctx.FormInt64("run_number"),
		CommitSHA:   ctx.FormString("head_sha"),
		Ref:         ctx.FormString("ref"),
		WorkflowID:  ctx.FormString("workflow_id"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListActionRuns", err)
		return
	}

	res := new(api.ListActionRunResponse)
	res.TotalCount = total

	res.Entries = make([]*api.ActionRun, len(runs))
	for i, r := range runs {
		if err := r.LoadAttributes(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
			return
		}
		cr := convert.ToActionRun(ctx, r, ctx.Doer())
		res.Entries[i] = cr
	}

	ctx.JSON(http.StatusOK, &res)
}

// GetActionRun get one action instance
func GetActionRun(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run_id} repository ActionRun
	// ---
	// summary: Get an action run
	// produces:
	// - application/json
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
	// - name: run_id
	//   in: path
	//   description: id of the action run
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionRun"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run, err := actions_model.GetRunByID(ctx, ctx.ParamsInt64(":run_id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunById", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRunByID", err)
		}
		return
	}

	// Action runs lives in its own table, therefore we check that the
	// run with the requested ID is owned by the repository
	if ctx.Repo().Repository.ID != run.RepoID {
		ctx.Error(http.StatusNotFound, "GetRunById", util.ErrNotExist)
		return
	}

	if err := run.LoadAttributes(ctx); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToActionRun(ctx, run, ctx.Doer()))
}

// DeleteActionRun removes a completed workflow run.
func DeleteActionRun(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/runs/{run_id} repository DeleteActionRun
	// ---
	// summary: Delete a completed workflow run.
	// description: >
	//   Remove a particular workflow run. The workflow run must have completed (succeeded, failed, cancelled) for the
	//   operation to succeed. Otherwise, an error is returned.
	// produces:
	// - application/json
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
	// - name: run_id
	//   in: path
	//   description: id of the action run
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: Workflow run has been removed
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run, err := actions_model.GetRunByID(ctx, ctx.ParamsInt64(":run_id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunById", err)
			return
		}

		ctx.Error(http.StatusInternalServerError, "GetRunByID", err)
		return
	}

	if ctx.Repo().Repository.ID != run.RepoID {
		ctx.Error(http.StatusNotFound, "GetRunById", util.ErrNotExist)
		return
	}

	err = actions_service.DeleteRun(ctx, run.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteRun", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CancelActionRun cancels a pending or running workflow run.
func CancelActionRun(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel repository CancelActionRun
	// ---
	// summary: Cancel a pending or running workflow run.
	// description: >
	//   Cancel a particular workflow run. Pending or running jobs of the run are cancelled. A run that has
	//   already finished, whether cancelled, failed, skipped or succeeded, is left unchanged.
	//   In both cases the endpoint responds with HTTP 204.
	// produces:
	// - application/json
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
	// - name: run_id
	//   in: path
	//   description: ID of the workflow run
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: Workflow run has been cancelled
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run, err := actions_model.GetRunByID(ctx, ctx.ParamsInt64(":run_id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunById", err)
			return
		}

		ctx.Error(http.StatusInternalServerError, "GetRunByID", err)
		return
	}

	if ctx.Repo().Repository.ID != run.RepoID {
		ctx.Error(http.StatusNotFound, "GetRunById", util.ErrNotExist)
		return
	}

	err = actions_service.CancelRun(ctx, run)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CancelRun", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ListActionRunJobs return a filtered list of jobs that belong to a single workflow run
func ListActionRunJobs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs repository ListActionRunJobs
	// ---
	// summary: List jobs of a workflow run
	// produces:
	// - application/json
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
	// - name: run_id
	//   in: path
	//   description: ID of the workflow run
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionRunJobList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	run, err := actions_model.GetRunByID(ctx, ctx.ParamsInt64(":run_id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunById", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRunByID", err)
		}
		return
	}

	// Action runs lives in its own table, therefore we check that the
	// run with the requested ID is owned by the repository
	if ctx.Repo().Repository.ID != run.RepoID {
		ctx.Error(http.StatusNotFound, "GetRunById", util.ErrNotExist)
		return
	}

	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRunJobsByRunID", err)
		return
	}

	response := make([]*api.ActionRunJob, 0, len(jobs))
	for _, job := range jobs {
		response = append(response, convert.ToActionRunJob(job, nil))
	}

	ctx.JSON(http.StatusOK, response)
}

// ListActionArtifacts list artifacts for a repository
func ListActionArtifacts(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts repository ListActionArtifacts
	// ---
	// summary: List a repository's artifacts
	// produces:
	// - application/json
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
	// - name: name
	//   in: query
	//   description: filter by artifact name
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionArtifactList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	opts := actions_model.FindArtifactsOptions{
		ListOptions:  utils.GetListOptions(ctx),
		RepoID:       ctx.Repo().Repository.ID,
		ArtifactName: ctx.FormString("name"),
	}

	arts, total, err := actions_model.ListAggregatedArtifacts(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListAggregatedArtifacts", err)
		return
	}

	repoAPIURL := ctx.Repo().Repository.APIURL()

	entries := make([]*api.ActionArtifact, len(arts))
	for i, art := range arts {
		entries[i] = convert.ToActionArtifact(repoAPIURL, art)
	}

	ctx.SetLinkHeader(int(total), opts.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, entries)
}

// ListActionRunArtifacts list artifacts for a workflow run
func ListActionRunArtifacts(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run_id}/artifacts repository ListActionRunArtifacts
	// ---
	// summary: List artifacts of a workflow run
	// produces:
	// - application/json
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
	// - name: run_id
	//   in: path
	//   description: ID of the workflow run
	//   type: integer
	//   format: int64
	//   required: true
	// - name: name
	//   in: query
	//   description: filter by artifact name
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionArtifactList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID := ctx.ParamsInt64(":run_id")
	run, err := actions_model.GetRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRunByID", err)
		}
		return
	}
	if ctx.Repo().Repository.ID != run.RepoID {
		ctx.Error(http.StatusNotFound, "GetRunByID", util.ErrNotExist)
		return
	}

	opts := actions_model.FindArtifactsOptions{
		ListOptions:  utils.GetListOptions(ctx),
		RunID:        runID,
		ArtifactName: ctx.FormString("name"),
	}

	arts, total, err := actions_model.ListAggregatedArtifacts(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListAggregatedArtifacts", err)
		return
	}

	repoAPIURL := ctx.Repo().Repository.APIURL()

	entries := make([]*api.ActionArtifact, len(arts))
	for i, art := range arts {
		entries[i] = convert.ToActionArtifact(repoAPIURL, art)
	}

	ctx.SetLinkHeader(int(total), opts.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, entries)
}

// GetActionArtifact get an artifact by its ID
func GetActionArtifact(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id} repository GetActionArtifact
	// ---
	// summary: Get an artifact by ID
	// produces:
	// - application/json
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
	// - name: artifact_id
	//   in: path
	//   description: ID of the artifact
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionArtifact"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	meta, err := actions_model.GetAggregatedArtifactByID(ctx, ctx.Repo().Repository.ID, ctx.ParamsInt64(":artifact_id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetAggregatedArtifactByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetAggregatedArtifactByID", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToActionArtifact(ctx.Repo().Repository.APIURL(), meta))
}

// DownloadActionArtifact download an artifact by its ID
func DownloadActionArtifact(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip repository DownloadActionArtifact
	// ---
	// summary: Download an artifact
	// produces:
	// - application/zip
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
	// - name: artifact_id
	//   in: path
	//   description: ID of the artifact
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     description: the artifact archive
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	meta, err := actions_model.GetAggregatedArtifactByID(ctx, ctx.Repo().Repository.ID, ctx.ParamsInt64(":artifact_id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetAggregatedArtifactByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetAggregatedArtifactByID", err)
		}
		return
	}

	// Load all artifact rows for this logical artifact
	artifacts, err := db.Find[actions_model.ActionArtifact](ctx, actions_model.FindArtifactsOptions{
		RunID:        meta.RunID,
		ArtifactName: meta.ArtifactName,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindArtifacts", err)
		return
	}

	for _, art := range artifacts {
		if art.Status != int64(actions_model.ArtifactStatusUploadConfirmed) {
			ctx.Error(http.StatusNotFound, "DownloadActionArtifact", errors.New("artifact not confirmed"))
			return
		}
	}

	if err := actions_service.ServeArtifact(ctx.Base, artifacts); err != nil {
		ctx.Error(http.StatusInternalServerError, "ServeArtifact", err)
		return
	}
}

// DeleteActionArtifact marks an artifact for deletion
func DeleteActionArtifact(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/actions/artifacts/{artifact_id} repository DeleteActionArtifact
	// ---
	// summary: Mark an artifact for deletion
	// description: |
	//   Marks the artifact for deletion. Storage space will be reclaimed
	//   asynchronously by a background job.
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
	// - name: artifact_id
	//   in: path
	//   description: ID of the artifact
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: artifact marked for deletion
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	meta, err := actions_model.GetAggregatedArtifactByID(ctx, ctx.Repo().Repository.ID, ctx.ParamsInt64(":artifact_id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetAggregatedArtifactByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetAggregatedArtifactByID", err)
		}
		return
	}

	if err := actions_model.SetArtifactNeedDelete(ctx, meta.RunID, meta.ArtifactName); err != nil {
		ctx.Error(http.StatusInternalServerError, "SetArtifactNeedDelete", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func GetActionJob(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/jobs/{job_id} repository repoGetActionJob
	// ---
	// summary: Get a single workflow run job, including its step list
	// description: >
	//   Returns the workflow job plus the ordered list of steps that make up
	//   its execution. The `number` field on each step is the value accepted
	//   by the `?step=` filter on the job-logs endpoint; `number=0` is always
	//   "Set up job" and the last index is always "Complete job", with the
	//   workflow's real steps in between. A job that has never been picked up
	//   by a runner returns the job metadata with `steps` omitted.
	// produces:
	// - application/json
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
	// - name: job_id
	//   in: path
	//   description: ID of the workflow job
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActionRunJob"
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	jobID := ctx.ParamsInt64(":job_id")

	job, err := actions_model.GetRunJobByID(ctx, jobID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunJobByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRunJobByID", err)
		}
		return
	}
	if job.RepoID != ctx.Repo().Repository.ID {
		ctx.Error(http.StatusNotFound, "GetRunJobByID", util.ErrNotExist)
		return
	}

	var task *actions_model.ActionTask
	if job.TaskID != 0 {
		task, err = actions_model.GetTaskByID(ctx, job.TaskID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetTaskByID", err)
			return
		}
	}

	var full []*actions_model.ActionTaskStep
	if task != nil {
		// Targeted step load — LoadAttributes pulls job + run too, which we
		// don't need here.
		steps, err := actions_model.GetTaskStepsByTaskID(ctx, task.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetTaskStepsByTaskID", err)
			return
		}
		task.Steps = steps
		full = actions.FullSteps(task)
	}

	ctx.JSON(http.StatusOK, convert.ToActionRunJob(job, full))
}

// GetActionJobLogs serves plaintext logs for a single action job.
func GetActionJobLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs repository repoGetActionJobLogs
	// ---
	// summary: Download the plaintext logs of an action job
	// description: >
	//   Returns the plaintext log for the job. By default the log for the
	//   most recent attempt is returned (ActionRunJob.TaskID tracks the latest
	//   task). Pass `?attempt=N` to fetch the log for a specific historical
	//   attempt; the value matches the `attempt` field returned by the job
	//   listing endpoints. Pass `?step=N` to narrow to a single step (using
	//   the `number` field from `GET /actions/jobs/{job_id}`). Pass
	//   `?format=ndjson` for NDJSON output (one `{time, content}` object per
	//   line); the NDJSON response drops `Accept-Ranges` and does not support
	//   `Range:` since the total length isn't known until the scan finishes.
	// produces:
	// - text/plain
	// - application/x-ndjson
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
	// - name: job_id
	//   in: path
	//   description: ID of the workflow job
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attempt
	//   in: query
	//   description: 1-based attempt number matching the value of `attempt` in the job listing; omit to fetch the latest attempt of the job
	//   type: integer
	//   format: int64
	//   required: false
	// - name: step
	//   in: query
	//   description: >
	//     Step number to filter to. Use the `number` field from
	//     `GET /repos/{owner}/{repo}/actions/jobs/{job_id}` — 0 is the
	//     "Set up job" entry, the last index is the "Complete job" entry,
	//     real steps are in between. Returns only that step's slice of
	//     the log. Range requests still work over the slice.
	//   type: integer
	//   required: false
	// - name: format
	//   in: query
	//   description: >
	//     Response shape. `text` (default) returns log content as plaintext
	//     lines. `ndjson` returns one `{"time":"...","content":"..."}` object
	//     per emitted line, separated by `\n` (content-type
	//     `application/x-ndjson`). `format=ndjson` drops `Accept-Ranges` and
	//     disables `Range:` support.
	//   type: string
	//   enum: [text, ndjson]
	//   required: false
	// responses:
	//   "200":
	//     description: Plaintext log content (or NDJSON when `format=ndjson`)
	//     schema:
	//       type: string
	//   "206":
	//     description: Partial log content (Range request; not returned when `format=ndjson` is set)
	//     schema:
	//       type: string
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	jobID := ctx.ParamsInt64(":job_id")

	var attempt optional.Option[int64]
	if ctx.FormString("attempt") != "" {
		attempt = optional.Some(ctx.FormInt64("attempt"))
	}

	var stepFilter optional.Option[int]
	if ctx.FormString("step") != "" {
		stepFilter = optional.Some(ctx.FormInt("step"))
	}

	asJSON := ctx.FormString("format") == "ndjson"

	reader, filename, modtime, err := actions_service.OpenJobLogReader(ctx, ctx.Repo().Repository, jobID, attempt, stepFilter)
	if err != nil {
		switch {
		case errors.Is(err, util.ErrNotExist),
			errors.Is(err, actions_service.ErrJobNotExecuted),
			errors.Is(err, actions_service.ErrLogsExpired),
			errors.Is(err, actions_service.ErrStepOutOfRange):
			ctx.Error(http.StatusNotFound, "OpenJobLogReader", err)
		default:
			ctx.Error(http.StatusInternalServerError, "OpenJobLogReader", err)
		}
		return
	}
	defer reader.Close()

	// JSON output: bypass http.ServeContent (no Range, no Content-Length)
	// and stream NDJSON via WriteJobLogStream.
	if asJSON {
		ctx.Resp.Header().Set("Content-Type", "application/x-ndjson")
		ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
		if err := actions_service.WriteJobLogStream(ctx.Resp, reader, actions_service.JobLogFilterOptions{JSON: true}); err != nil {
			// Best-effort — headers may already be flushed, so we can only log.
			log.Error("WriteJobLogStream job %d: %v", jobID, err)
		}
		return
	}

	ctx.Resp.Header().Set("Accept-Ranges", "bytes")
	// Pin Content-Type explicitly so http.ServeContent doesn't extension-sniff.
	// On Linux with shared-mime-info installed, mime.TypeByExtension(".log")
	// returns "text/x-log; charset=utf-8" — a non-IANA type whose `x-` prefix
	// is deprecated by RFC 6648 and which varies by host (macOS or minimal
	// containers return empty or application/octet-stream). Our swagger
	// documents `produces: text/plain`, and a client sending
	// `Accept: text/plain` would 406-mismatch on the unpinned response.
	ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeContent(ctx.Resp, ctx.Req, filename, modtime, reader)
}

// GetActionRunLogs streams a ZIP of plaintext logs for every job in the run.
func GetActionRunLogs(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/runs/{run_id}/logs repository repoGetActionRunLogs
	// ---
	// summary: Download a ZIP of plaintext logs for every job in an action run
	// produces:
	// - application/zip
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
	// - name: run_id
	//   in: path
	//   description: >
	//     ID of the workflow run. The ZIP contains the latest attempt of each
	//     job in the run, with each entry named
	//     `{job-name}-{job-id}-attempt-{N}.log` (the job ID prevents collisions
	//     when two jobs share a name; the attempt number records which run the
	//     log came from). The run itself has no attempt number — jobs are
	//     re-run independently, so use the per-job logs endpoint with `?attempt`
	//     to fetch a specific historical attempt of one job.
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     description: ZIP archive of per-job log files
	//     schema:
	//       type: string
	//       format: binary
	//   "401":
	//     "$ref": "#/responses/unauthorized"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	runID := ctx.ParamsInt64(":run_id")

	run, err := actions_model.GetRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRunByID", err)
		}
		return
	}

	if run.RepoID != ctx.Repo().Repository.ID {
		ctx.Error(http.StatusNotFound, "GetRunByID", util.ErrNotExist)
		return
	}

	zipFilename := fmt.Sprintf("run-%d-logs.zip", run.ID)

	ctx.Resp.Header().Set("Content-Type", "application/zip")
	ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))

	if err := actions_service.WriteRunLogsZip(ctx, ctx.Resp, run); err != nil {
		ctx.Error(http.StatusInternalServerError, "WriteRunLogsZip", err)
		return
	}
}
