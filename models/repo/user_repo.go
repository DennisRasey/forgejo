// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"slices"
	"strings"

	"forgejo.org/models/db"
	"forgejo.org/models/perm"
	"forgejo.org/models/unit"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/container"
	api "forgejo.org/modules/structs"

	"xorm.io/builder"
)

// GetRepoAssignees returns all users that have write access and can be assigned to issues
// of the repository,
func GetRepoAssignees(ctx context.Context, repo *Repository) (_ []*user_model.User, err error) {
	if err = repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	e := db.GetEngine(ctx)
	userIDs := make([]int64, 0, 10)
	if err = e.Table("access").
		Where("repo_id = ? AND mode >= ?", repo.ID, perm.AccessModeWrite).
		Select("user_id").
		Find(&userIDs); err != nil {
		return nil, err
	}

	uniqueUserIDs := make(container.Set[int64])
	uniqueUserIDs.AddMultiple(userIDs...)

	if repo.Owner.IsOrganization() {
		additionalUserIDs := make([]int64, 0, 10)
		if err = e.Table("team_user").
			Join("INNER", "team_repo", "`team_repo`.team_id = `team_user`.team_id").
			Join("INNER", "team_unit", "`team_unit`.team_id = `team_user`.team_id").
			Where("`team_repo`.repo_id = ? AND (`team_unit`.access_mode >= ? OR (`team_unit`.access_mode = ? AND `team_unit`.`type` = ?))",
				repo.ID, perm.AccessModeWrite, perm.AccessModeRead, unit.TypePullRequests).
			Distinct("`team_user`.uid").
			Select("`team_user`.uid").
			Find(&additionalUserIDs); err != nil {
			return nil, err
		}
		uniqueUserIDs.AddMultiple(additionalUserIDs...)
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*user_model.User, 0, len(uniqueUserIDs)+1)
	if len(uniqueUserIDs) > 0 {
		if err = e.In("id", uniqueUserIDs.Values()).
			Where(builder.Eq{"`user`.is_active": true}).
			OrderBy(user_model.GetOrderByName()).
			Find(&users); err != nil {
			return nil, err
		}
	}
	if !repo.Owner.IsOrganization() && !uniqueUserIDs.Contains(repo.OwnerID) {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// GetReviewers get all users can be requested to review:
// * for private repositories this returns all users that have read access or higher to the repository.
// * for public repositories this returns all users that have read access or higher to the repository,
// all repo watchers and all organization members.
// TODO: may be we should have a busy choice for users to block review request to them.
func GetReviewers(ctx context.Context, repo *Repository, posterID int64) ([]*user_model.User, error) {
	// Get the owner of the repository - this often already pre-cached and if so saves complexity for the following queries
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	candidateQueries := []string{
		`SELECT user_id AS id FROM access WHERE repo_id = ? AND mode >= ?`,
	}
	candidateArgs := []any{repo.ID, perm.AccessModeRead}

	if repo.IsPrivate || repo.Owner.Visibility == api.VisibleTypePrivate {
		// If the repository is private and owned by an individual user, add the
		// owner's user ID as a reviewer canidate
		if repo.Owner.Type == user_model.UserTypeIndividual && repo.Owner.ID != posterID {
			candidateQueries = append(candidateQueries, `SELECT ? AS id`)
			candidateArgs = append(candidateArgs, repo.Owner.ID)
		}
	} else {
		// If the repository is public, add all user IDs of users watching pull
		// requests and members of the organization the repository is part of.
		// Since organizations are a superset of users and are stored in the same
		// table with a different user type, uncoditionally using the repository
		// owner ID will return no users when it is of an individual user.
		candidateQueries = append(
			candidateQueries,
			`SELECT user_id AS id FROM watch WHERE repo_id = ? AND watch_selection_pull_requests = true`,
			`SELECT uid AS id FROM org_user WHERE org_id = ?`,
		)
		candidateArgs = append(candidateArgs, repo.ID, repo.OwnerID)
	}

	// Filter out inactive users and the user who created the pull request
	cond := builder.And(
		builder.Neq{"`user`.id": posterID},
		builder.Eq{"`user`.is_active": true},
	)

	query, condArgs, err := builder.Select("`user`.*").
		// Union the SQL queries which have been constructed above, returning a
		// deduplicated(!) list of user IDs
		From("("+strings.Join(candidateQueries, " UNION ")+")", "AS candidate").
		Join("INNER", "`user`", "`user`.id = candidate.id").
		Where(cond).
		OrderBy(user_model.GetOrderByName()).
		ToSQL()
	if err != nil {
		return nil, err
	}

	args := slices.Concat(candidateArgs, condArgs)
	users := make([]*user_model.User, 0, 8)
	err = db.GetEngine(ctx).SQL(query, args...).Find(&users)
	return users, err
}

// GetIssuePostersWithSearch returns up to 30 users whose username starts with or full_name contains the given search string for the given repository.
func GetIssuePostersWithSearch(ctx context.Context, repo *Repository, isPull bool, search string) ([]*user_model.User, error) {
	users := make([]*user_model.User, 0, 30)
	prefixCond := builder.Or(
		db.BuildCaseInsensitiveLike("name", search+"%"),
		db.BuildCaseInsensitiveLike("full_name", "%"+search+"%"),
	)

	cond := builder.In("`user`.id",
		builder.Select("poster_id").From("issue").Where(
			builder.Eq{"repo_id": repo.ID}.
				And(builder.Eq{"is_pull": isPull}),
		).GroupBy("poster_id")).And(prefixCond)

	return users, db.GetEngine(ctx).
		Where(cond).
		Cols("id", "name", "full_name", "avatar", "avatar_email", "use_custom_avatar").
		OrderBy("name").
		Limit(30).
		Find(&users)
}

// GetWatchedRepoIDsOwnedBy returns the repos owned by a particular user watched by a particular user
func GetWatchedRepoIDsOwnedBy(ctx context.Context, userID, ownedByUserID int64) ([]int64, error) {
	repoIDs := make([]int64, 0, 10)
	err := db.GetEngine(ctx).
		Table("repository").
		Select("`repository`.id").
		Join("LEFT", "watch", "`repository`.id=`watch`.repo_id").
		Where("`watch`.user_id=?", userID).
		And(BuilderWatchAnything()).
		And("`repository`.owner_id=?", ownedByUserID).Find(&repoIDs)
	return repoIDs, err
}
