// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/util"
	"forgejo.org/modules/web"
	"forgejo.org/routers/api/v1/utils"
	"forgejo.org/services/context"
	"forgejo.org/services/convert"

	gouuid "github.com/google/uuid"
)

// RegistrationToken is a string used to register a runner with a server
type RegistrationToken struct {
	Token string `json:"token"`
}

func GetRegistrationToken(ctx *context.APIContext, ownerID, repoID int64) {
	optOwnerID := optional.None[int64]()
	if ownerID != 0 {
		optOwnerID = optional.Some(ownerID)
	}
	optRepoID := optional.None[int64]()
	if repoID != 0 {
		optRepoID = optional.Some(repoID)
	}

	token, err := actions_model.GetLatestRunnerToken(ctx, optOwnerID, optRepoID)
	if errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, optOwnerID, optRepoID)
	}
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, RegistrationToken{Token: token.Token})
}

func GetActionRunJobs(ctx *context.APIContext, ownerID, repoID int64) {
	labels := []string{}
	if len(ctx.Req.Form["labels"]) > 0 {
		labels = strings.Split(ctx.FormTrim("labels"), ",")
	}

	total, err := db.Find[actions_model.ActionRunJob](ctx, &actions_model.FindTaskOptions{
		Status:  []actions_model.Status{actions_model.StatusWaiting, actions_model.StatusRunning},
		OwnerID: ownerID,
		RepoID:  repoID,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountWaitingActionRunJobs", err)
		return
	}

	res := fromRunJobModelToResponse(total, labels)

	ctx.JSON(http.StatusOK, res)
}

func fromRunJobModelToResponse(job []*actions_model.ActionRunJob, labels []string) []*structs.ActionRunJob {
	var res []*structs.ActionRunJob
	for i := range job {
		if len(labels) == 0 || labels[0] == "" && len(job[i].RunsOn) == 0 || job[i].ItRunsOn(labels) {
			res = append(res, convert.ToActionRunJob(job[i], nil))
		}
	}
	return res
}

// ListRunners lists runners for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means all runners including global runners, does not appear in sql where clause
// ownerID == 0 and repoID != 0 means all runners for the given repo
// ownerID != 0 and repoID == 0 means all runners for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func ListRunners(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("ownerID and repoID should not be both set: %d and %d", ownerID, repoID))
		return
	}

	listOptions := utils.GetListOptions(ctx)
	runners, total, err := db.FindAndCount[actions_model.ActionRunner](ctx, &actions_model.FindRunnerOptions{
		OwnerID:     ownerID,
		RepoID:      repoID,
		ListOptions: listOptions,
		WithVisible: ctx.FormBool("visible"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindCountRunners", map[string]string{})
		return
	}

	runnerList := make([]structs.ActionRunner, len(runners))
	for i, runner := range runners {
		actionRunner, err := convert.ToActionRunner(runner)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ToActionRunner", err)
			return
		}
		runnerList[i] = actionRunner
	}

	ctx.SetLinkHeader(int(total), listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, &runnerList)
}

// GetRunner get the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func GetRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("ownerID and repoID should not be both set: %d and %d", ownerID, repoID))
		return
	}
	runner, err := actions_model.GetVisibleRunnerByID(ctx, runnerID, ownerID, repoID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "GetRunnerNotFound", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRunnerFailed", err)
		}
		return
	}

	actionRunner, err := convert.ToActionRunner(runner)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ToActionRunner", err)
		return
	}
	ctx.JSON(http.StatusOK, actionRunner)
}

func RegisterRunner(ctx *context.APIContext, ownerID, repoID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "RegisterRunner", fmt.Errorf("ownerID '%d' and repoID '%d' cannot be set simultaneously", ownerID, repoID))
		return
	}

	options := web.GetForm(ctx).(*structs.RegisterRunnerOptions)
	runner := &actions_model.ActionRunner{
		UUID:        gouuid.NewString(),
		Name:        options.Name,
		OwnerID:     ownerID,
		RepoID:      repoID,
		Description: options.Description,
		Ephemeral:   options.Ephemeral,
	}
	runner.GenerateToken()
	if err := actions_model.CreateRunner(ctx, runner); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateRunner", err)
		return
	}

	response := &structs.RegisterRunnerResponse{
		ID:    runner.ID,
		UUID:  runner.UUID,
		Token: runner.Token,
	}
	ctx.JSON(http.StatusCreated, response)
}

// DeleteRunner deletes the runner for api route validated ownerID and repoID
// ownerID == 0 and repoID == 0 means any runner including global runners
// ownerID == 0 and repoID != 0 means any runner for the given repo
// ownerID != 0 and repoID == 0 means any runner for the given user/org
// ownerID != 0 and repoID != 0 undefined behavior
// Access rights are checked at the API route level
func DeleteRunner(ctx *context.APIContext, ownerID, repoID, runnerID int64) {
	if ownerID != 0 && repoID != 0 {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("ownerID and repoID should not be both set: %d and %d", ownerID, repoID))
		return
	}
	runner, err := actions_model.GetVisibleRunnerByID(ctx, runnerID, ownerID, repoID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.Error(http.StatusNotFound, "DeleteRunnerNotFound", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteRunnerFailed", err)
		}
		return
	}
	if !runner.Editable(ownerID, repoID) {
		ctx.Error(http.StatusNotFound, "EditRunner", "No permission to delete this runner")
		return
	}

	err = actions_model.DeleteRunner(ctx, runner)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
