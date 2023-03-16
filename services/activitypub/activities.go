// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/forgefed"

	ap "github.com/go-ap/activitypub"
)

// Create Follow activity
func Follow(actorUser, followUser *user_model.User) *ap.Follow {
	return &ap.Follow{
		Type:   ap.FollowType,
		Actor:  ap.PersonNew(ap.IRI(actorUser.GetIRI())),
		Object: ap.PersonNew(ap.IRI(followUser.GetIRI())),
		To:     ap.ItemCollection{ap.IRI(followUser.GetIRI())},
	}
}

// Create Undo Follow activity
func Unfollow(actorUser, followUser *user_model.User) *ap.Undo {
	return &ap.Undo{
		Type:   ap.UndoType,
		Actor:  ap.PersonNew(ap.IRI(actorUser.GetIRI())),
		Object: Follow(actorUser, followUser),
		To:     ap.ItemCollection{ap.IRI(followUser.GetIRI())},
	}
}

// Create Like activity
func Star(user *user_model.User, repo *repo_model.Repository) *ap.Like {
	return &ap.Like{
		Type:   ap.LikeType,
		Actor:  ap.PersonNew(ap.IRI(user.GetIRI())),
		Object: forgefed.RepositoryNew(ap.IRI(repo.GetIRI())),
		To:     ap.ItemCollection{ap.IRI(repo.GetIRI())},
	}
}

// Create Undo Like activity
func Unstar(user *user_model.User, repo *repo_model.Repository) *ap.Undo {
	return &ap.Undo{
		Type:   ap.UndoType,
		Actor:  ap.PersonNew(ap.IRI(user.GetIRI())),
		Object: Star(user, repo),
		To:     ap.ItemCollection{ap.IRI(repo.GetIRI())},
	}
}

// Create Create activity
func Create(user *user_model.User, object ap.ObjectOrLink, to string) *ap.Create {
	return &ap.Create{
		Type:   ap.CreateType,
		Actor:  ap.PersonNew(ap.IRI(user.GetIRI())),
		Object: object,
		To:     ap.ItemCollection{ap.IRI(to)},
	}
}
