// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
)

// StarRepo or unstar repository.
func StarRepo(ctx context.Context, userID, repoID int64, star bool) error {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return err
	}
	err = repo.LoadOwner(ctx)
	if err != nil {
		return err
	}
	if repo.Owner.LoginType == auth.Federated {
		// Federated repo
		user, err := user_model.GetUserByID(ctx, userID)
		if err != nil {
			return err
		}
		var activity *ap.Activity
		if star {
			activity = activitypub.Star(user, repo)
		} else {
			activity = activitypub.Unstar(user, repo)
		}
		err = activitypub.Send(ctx, user, activity)
		if err != nil {
			return err
		}
	}
	err = repo_model.StarRepo(ctx, userID, repoID, star)
	if err != nil {
		return err
	}
	user, err := user_model.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	note := ap.Note{
		Type:         ap.NoteType,
		ID:           ap.IRI(repo.GetIRI()), // TODO: serve the note at an API endpoint
		AttributedTo: ap.IRI(user.GetIRI()),
		To:           ap.ItemCollection{ap.IRI("https://www.w3.org/ns/activitystreams#Public")},
	}
	note.Content = ap.NaturalLanguageValuesNew()
	err = note.Content.Set("en", ap.Content(user.Name+" starred <a href=\""+repo.HTMLURL()+"\">"+repo.FullName()+"</a>"))
	if err != nil {
		return err
	}
	create := ap.Create{
		Type:   ap.CreateType,
		Actor:  ap.PersonNew(ap.IRI(user.GetIRI())),
		Object: note,
		To:     ap.ItemCollection{ap.IRI(user.GetIRI() + "/followers")},
	}
	return activitypub.Send(ctx, user, &create)
}
