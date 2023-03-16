// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"context"

	user_model "code.gitea.io/gitea/models/user"
	user_service "code.gitea.io/gitea/services/user"

	ap "github.com/go-ap/activitypub"
)

// Process an incoming Delete activity
func delete(ctx context.Context, delete ap.Delete) error {
	actorIRI := delete.Actor.GetLink()
	objectIRI := delete.Object.GetLink()
	// Make sure actor matches the object getting deleted
	if actorIRI != objectIRI {
		return nil
	}

	// Object is the user getting deleted
	objectUser, err := user_model.GetUserByIRI(ctx, objectIRI.String())
	if err != nil {
		return err
	}
	return user_service.DeleteUser(ctx, objectUser, true)
}
