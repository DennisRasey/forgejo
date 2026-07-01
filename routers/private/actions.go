// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	gocontext "context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	actions_model "forgejo.org/models/actions"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/json"
	"forgejo.org/modules/log"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/private"
	"forgejo.org/modules/util"
	"forgejo.org/services/context"
)

// GenerateActionsRunnerToken generates a new runner token for a given scope
func GenerateActionsRunnerToken(ctx *context.PrivateContext) {
	var genRequest private.GenerateTokenRequest
	rd := ctx.Req.Body
	defer rd.Close()

	if err := json.NewDecoder(rd).Decode(&genRequest); err != nil {
		log.Error("JSON Decode failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	owner, repo, err := ParseScope(ctx, genRequest.Scope)
	if err != nil {
		log.Error("parseScope failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	ownerID := optional.None[int64]()
	if owner != 0 {
		ownerID = optional.Some(owner)
	}
	repoID := optional.None[int64]()
	if repo != 0 {
		repoID = optional.Some(repo)
	}

	token, err := actions_model.GetLatestRunnerToken(ctx, ownerID, repoID)
	if errors.Is(err, util.ErrNotExist) || (token != nil && !token.IsActive) {
		token, err = actions_model.NewRunnerToken(ctx, ownerID, repoID)
		if err != nil {
			errMsg := fmt.Sprintf("error while creating runner token: %v", err)
			log.Error("NewRunnerToken failed: %v", errMsg)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: errMsg,
			})
			return
		}
	} else if err != nil {
		errMsg := fmt.Sprintf("could not get unactivated runner token: %v", err)
		log.Error("GetLatestRunnerToken failed: %v", errMsg)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: errMsg,
		})
		return
	}

	ctx.PlainText(http.StatusOK, token.Token)
}

func ParseScope(ctx gocontext.Context, scope string) (ownerID, repoID int64, err error) {
	if scope == "" {
		return 0, 0, nil
	}

	ownerName, repoName, found := strings.Cut(scope, "/")

	u, err := user_model.GetUserByName(ctx, ownerName)
	if err != nil {
		return 0, 0, err
	}

	if !found {
		return u.ID, 0, nil
	}

	r, err := repo_model.GetRepositoryByName(ctx, u.ID, repoName)
	if err != nil {
		return 0, 0, err
	}
	return 0, r.ID, nil
}
