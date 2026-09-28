// Copyright 2022 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"

	issues_model "forgejo.org/models/issues"
	access_model "forgejo.org/models/perm/access"
	unit_model "forgejo.org/models/unit"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/util"
)

var (
	ErrUserHasNoPermissionForAction = errors.New("user not allowed to do this action")
	// ErrHeadBranchNotEditable is returned when the head branch cannot receive commits.
	ErrHeadBranchNotEditable = errors.New("pull request head branch is not editable")
)

// CheckHeadBranchEditable returns nil when doer may push to the head branch of pr
// ErrHeadBranchNotEditable when the pull request state forbids it
// util.ErrPermissionDenied when doer lacks the rights.
func CheckHeadBranchEditable(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) error {
	if doer == nil {
		return util.ErrPermissionDenied
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return err
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return err
	}
	if pr.HeadRepo == nil || pr.HasMerged || pr.Issue.IsClosed || pr.Flow == issues_model.PullRequestFlowAGit || !pr.HeadRepo.CanEnableEditor() {
		return ErrHeadBranchNotEditable
	}
	perm, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
	if err != nil {
		return err
	}
	if !issues_model.CanMaintainerWriteToBranch(ctx, perm, pr.HeadBranch, doer, access_model.GetUserRepoPermission) {
		return util.ErrPermissionDenied
	}
	return nil
}

// CanEditHeadBranch is CheckHeadBranchEditable as a boolean; only unexpected errors are returned.
func CanEditHeadBranch(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) (bool, error) {
	err := CheckHeadBranchEditable(ctx, doer, pr)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrHeadBranchNotEditable), errors.Is(err, util.ErrPermissionDenied):
		return false, nil
	default:
		return false, err
	}
}

// SetAllowEdits allow edits from maintainers to PRs
func SetAllowEdits(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest, allow bool) error {
	if doer == nil || !pr.Issue.IsPoster(doer.ID) {
		return ErrUserHasNoPermissionForAction
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		return err
	}

	permission, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
	if err != nil {
		return err
	}

	if !permission.CanWrite(unit_model.TypeCode) {
		return ErrUserHasNoPermissionForAction
	}

	pr.AllowMaintainerEdit = allow
	return issues_model.UpdateAllowEdits(ctx, pr)
}
