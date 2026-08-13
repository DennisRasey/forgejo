// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package authz

import (
	"context"
	"fmt"

	"forgejo.org/models/perm"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/structs"

	"xorm.io/builder"
)

// For specific repositories listed in [AccessTokenResourceRepo] models, all access is permitted.  For public
// repositories that aren't listed among the specific repos, read-only access is permitted.  For all other repos, no
// access is permitted.
type SpecificReposAuthorizationReducer struct {
	ResourceRepos []RepoGetter
}

type RepoGetter interface {
	GetTargetRepoID() int64
}

func (r *SpecificReposAuthorizationReducer) ReduceRepoAccess(ctx context.Context, repo *repo_model.Repository, accessMode perm.AccessMode) (perm.AccessMode, error) {
	for _, tokenRepo := range r.ResourceRepos {
		if tokenRepo.GetTargetRepoID() == repo.ID {
			// No restrictions as this repo is within the scope of the access token.
			return accessMode, nil
		}
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return 0, fmt.Errorf("failed to LoadOwner during ReduceRepoAccess: %w", err)
	}

	// Fine-grained access tokens remove access to any private repositories, or repository owned by non-public users,
	// that aren't listed in their resource list.
	if !repo.Owner.Visibility.IsPublic() || repo.IsPrivate {
		return perm.AccessModeNone, nil
	}

	// Public repos will be reduced to read access.
	return min(accessMode, perm.AccessModeRead), nil
}

func (r *SpecificReposAuthorizationReducer) RepoReadAccessFilter() builder.Cond {
	repoIDs := make([]int64, len(r.ResourceRepos))
	for i, tokenRepo := range r.ResourceRepos {
		repoIDs[i] = tokenRepo.GetTargetRepoID()
	}
	targetRepos := builder.In("repository.id", repoIDs)

	// We should also be able to see all non-private repositories that aren't in a private or limited organization.
	return builder.Or(
		targetRepos,
		builder.And(
			builder.Eq{"repository.is_private": false},
			builder.NotIn("repository.owner_id", builder.Select("id").From("`user`").Where(
				builder.Or(builder.Eq{"visibility": structs.VisibleTypeLimited}, builder.Eq{"visibility": structs.VisibleTypePrivate}),
			)),
		),
	)
}

func (*SpecificReposAuthorizationReducer) AllowAdminOverride() bool {
	return false
}
