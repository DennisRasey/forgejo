// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package context

import (
	"fmt"
	"net/http"

	project_model "forgejo.org/models/project"
	"forgejo.org/models/unit"
	"forgejo.org/modules/log"
)

func ReqProjectIDAssignableToIssueAndSetData(ctx *Context, projectID int64) {
	project := getProjectID(ctx, projectID)
	if ctx.Written() {
		return
	}
	reqProjectAssignableToIssue(ctx, project)
	if ctx.Written() {
		return
	}
	ctx.Data["Project"] = project
	ctx.Data["project_id"] = project.ID
}

func ReqProjectIDAssignableToIssue(ctx *Context, projectID int64) {
	project := getProjectID(ctx, projectID)
	if ctx.Written() {
		return
	}
	reqProjectAssignableToIssue(ctx, project)
}

func getProjectID(ctx *Context, projectID int64) *project_model.Project {
	project, err := project_model.GetProjectByID(ctx, projectID)
	if err != nil {
		if project_model.IsErrProjectNotExist(err) {
			ctx.NotFound(fmt.Sprintf("project %d is not found", projectID), nil)
			return nil
		}
		log.Error("project_model.GetProjectByID(%d): %v", projectID, err)
		ctx.ServerError(fmt.Sprintf("project_model.GetProjectByID(%d)", projectID), err)
		return nil
	}
	return project
}

func reqProjectAssignableToIssue(ctx *Context, project *project_model.Project) {
	reqValidAndConsistentProject(ctx, project)
	if ctx.Written() {
		return
	}
	reqPermissionToAssignProjectToIssue(ctx, project.Type)
}

func reqValidAndConsistentProject(ctx *Context, project *project_model.Project) {
	if project.RepoID != ctx.Repo.Repository.ID && project.OwnerID != ctx.Repo.Repository.OwnerID {
		ctx.NotFound(fmt.Sprintf("project %d does not belong", project.ID), nil)
	}
}

func reqPermissionToAssignProjectToIssue(ctx *Context, projectType project_model.Type) {
	switch projectType {
	case project_model.TypeRepository:
		if !ctx.Repo.Repository.UnitEnabled(ctx, unit.TypeProjects) {
			ctx.NotFound("repository projects are disabled", nil)
			return
		}
		if !ctx.Repo.CanRead(unit.TypeProjects) {
			ctx.Error(http.StatusForbidden, "doesn't have permissions to read repository projects")
			return
		}
		if !ctx.Repo.CanWrite(unit.TypeIssues) {
			ctx.Error(http.StatusForbidden, "doesn't have permissions to write repository issues")
			return
		}
	case project_model.TypeOrganization:
		if !ctx.Org.CanReadUnit(ctx, unit.TypeProjects) {
			ctx.Error(http.StatusForbidden, "doesn't have permissions to read the owner projects")
			return
		}
		if !ctx.Org.CanWriteUnit(ctx, unit.TypeIssues) {
			ctx.Error(http.StatusForbidden, "doesn't have permissions to write the repository issues and set the project")
			return
		}
	case project_model.TypeIndividual:
		if !ctx.Repo.CanWrite(unit.TypeIssues) {
			ctx.Error(http.StatusForbidden, "doesn't have permissions to write the repository issues and set the project")
			return
		}
	default:
		ctx.ServerError(fmt.Sprintf("unexpected project type %v", projectType), nil)
	}
}
