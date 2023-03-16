// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
)

func CreateFork(ctx context.Context, instance, username, reponame, destUsername string) error {
	// TODO: Clean this up

	// Migrate repository code
	user, err := user_model.GetUserByName(ctx, destUsername)
	if err != nil {
		return err
	}

	// _, err = migrations.MigrateRepository(ctx, user, destUsername, migrations.MigrateOptions{
	// 	CloneAddr: "https://" + instance + "/" + username + "/" + reponame + ".git",
	// 	RepoName:  reponame,
	// }, nil)
	// if err != nil {
	// 	return err
	// }

	// TODO: Make the migrated repo a fork

	// Send a Create activity to the instance we are forking from
	create := ap.Create{Type: ap.CreateType}
	create.To = ap.ItemCollection{ap.IRI("https://" + instance + "/api/v1/activitypub/repo/" + username + "/" + reponame)}
	repo := ap.IRI(setting.AppURL + "api/v1/activitypub/repo/" + destUsername + "/" + reponame)
	// repo := forgefed.RepositoryNew(ap.IRI(setting.AppURL + "api/v1/activitypub/repo/" + destUsername + "/" + reponame))
	// repo.ForkedFrom = forgefed.RepositoryNew(ap.IRI())
	create.Object = repo

	return activitypub.Send(ctx, user, &create)
}
