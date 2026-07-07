// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package repository_test

import (
	"io/fs"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/storage"
	repo_service "forgejo.org/services/repository"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/require"
)

func TestDeleteRepository(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	t.Run("Normal", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

		require.NoError(t, repo_service.DeleteRepository(t.Context(), user2, repo, false))

		unittest.AssertExistsIf(t, false, &repo_model.Repository{ID: 1})
	})

	t.Run("Foreign key reference", func(t *testing.T) {
		repo := forgery.CreateRepository(t, user2, nil)
		accessToken := forgery.CreateAccessToken(t, user2, &forgery.CreateAccessTokenOptions{
			RepositoryIDs: []int64{repo.ID},
		})

		require.NoError(t, repo_service.DeleteRepository(t.Context(), user2, repo, false))

		unittest.AssertExistsIf(t, true, &auth_model.AccessToken{ID: accessToken.ID})
		unittest.AssertExistsIf(t, false, &auth_model.AccessTokenResourceRepo{RepoID: repo.ID})
	})

	t.Run("Attachment", func(t *testing.T) {
		repo := forgery.CreateRepository(t, user2, nil)

		attachment := &repo_model.Attachment{
			RepoID: repo.ID,
			UUID:   "sleepy",
		}
		unittest.AssertSuccessfulInsert(t, attachment)
		storage.Attachments.Save("s/l/sleepy", strings.NewReader("Sleepy cat photo"), 16)

		require.NoError(t, repo_service.DeleteRepository(t.Context(), user2, repo, false))

		unittest.AssertExistsIf(t, false, &repo_model.Attachment{ID: attachment.ID})
		_, err := storage.Attachments.Stat("s/l/sleepy")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})
}
