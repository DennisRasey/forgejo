// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"context"
	"errors"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
)

// Process an incoming Follow activity
func follow(ctx context.Context, follow ap.Follow) error {
	// Actor is the user performing the follow
	actorIRI := follow.Actor.GetLink()
	actorUser, err := user_model.GetUserByIRI(ctx, actorIRI.String())
	if err != nil {
		return err
	}

	// Object is the user being followed
	objectIRI := follow.Object.GetLink()
	objectUser, err := user_model.GetUserByIRI(ctx, objectIRI.String())
	// Must be a local user
	if err != nil || strings.Contains(objectUser.Name, "@") {
		return err
	}

	err = user_model.FollowUser(ctx, actorUser.ID, objectUser.ID)
	if err != nil {
		return err
	}

	// Send back an Accept activity
	accept := ap.AcceptNew(objectIRI, follow)
	accept.Actor = ap.Person{ID: objectIRI}
	accept.To = ap.ItemCollection{ap.IRI(actorIRI.String())}
	accept.Object = follow
	return activitypub.Send(ctx, objectUser, accept)
}

// Process an incoming Undo follow activity
func unfollow(ctx context.Context, unfollow ap.Undo) error {
	// Object contains the follow
	follow, ok := unfollow.Object.(*ap.Follow)
	if !ok {
		return errors.New("could not cast object to follow")
	}

	// Actor is the user performing the undo follow
	actorIRI := follow.Actor.GetLink()
	actorUser, err := user_model.GetUserByIRI(ctx, actorIRI.String())
	if err != nil {
		return err
	}

	// Object is the user being unfollowed
	objectIRI := follow.Object.GetLink()
	objectUser, err := user_model.GetUserByIRI(ctx, objectIRI.String())
	// Must be a local user
	if err != nil || strings.Contains(objectUser.Name, "@") {
		return err
	}

	return user_model.UnfollowUser(ctx, actorUser.ID, objectUser.ID)
}
