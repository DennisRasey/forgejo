// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"context"
	"errors"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

	ap "github.com/go-ap/activitypub"
)

// Process a Like activity to star a repository
func star(ctx context.Context, like ap.Like) (err error) {
	user, err := user_model.GetUserByIRI(ctx, like.Actor.GetLink().String())
	if err != nil {
		return
	}
	repo, err := repo_model.GetRepositoryByIRI(ctx, like.Object.GetLink().String())
	if err != nil || strings.Contains(repo.Name, "@") || repo.IsPrivate {
		return
	}
	return repo_model.StarRepo(ctx, user.ID, repo.ID, true)
}

// Process an Undo Like activity to unstar a repository
func unstar(ctx context.Context, unlike ap.Undo) (err error) {
	like, ok := unlike.Object.(*ap.Like)
	if !ok {
		return errors.New("could not cast object to like")
	}
	user, err := user_model.GetUserByIRI(ctx, like.Actor.GetLink().String())
	if err != nil {
		return
	}
	repo, err := repo_model.GetRepositoryByIRI(ctx, like.Object.GetLink().String())
	if err != nil || strings.Contains(repo.Name, "@") || repo.IsPrivate {
		return
	}
	return repo_model.StarRepo(ctx, user.ID, repo.ID, false)
}
