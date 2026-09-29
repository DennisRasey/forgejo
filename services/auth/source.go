// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/optional"
)

// DeleteSource deletes a AuthSource record in DB.
func DeleteSource(ctx context.Context, source *auth.Source) error {
	count, err := db.GetEngine(ctx).Count(&user_model.User{LoginSource: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return auth.ErrSourceInUse{
			ID: source.ID,
		}
	}

	count, err = db.GetEngine(ctx).Count(&user_model.ExternalLoginUser{LoginSourceID: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return auth.ErrSourceInUse{
			ID: source.ID,
		}
	}

	if registerableSource, ok := source.Cfg.(auth.RegisterableSource); ok {
		if err := registerableSource.UnregisterSource(); err != nil {
			return err
		}
	}

	if _, err := db.GetEngine(ctx).Where("login_source_id = ?", source.ID).Delete(new(auth.AuthorizationToken)); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).
		Where("created_by_login_source_id = ?", source.ID).
		Cols("created_by_login_source_id").
		Update(organization.TeamUser{CreatedByLoginSourceID: optional.None[int64]()}); err != nil {
		return err
	}

	_, err = db.GetEngine(ctx).ID(source.ID).Delete(new(auth.Source))
	return err
}
