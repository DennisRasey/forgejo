// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgery

import (
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/require"
)

type CreateAccessTokenOptions struct {
	Name  string // if nil a unique name (derived from the test name) will be generated
	Scope string // scope of which resources the access token may access, if nil all scopes are granted.

	// If specified, this access token becomes repo-specific to specified repositories.
	RepositoryIDs []int64
}

func CreateAccessToken(t testing.TB, user *user_model.User, opts *CreateAccessTokenOptions) *auth_model.AccessToken {
	t.Helper()

	require.NotNil(t, user, "An existing user is required")
	if opts == nil {
		opts = &CreateAccessTokenOptions{}
	}

	if opts.Name == "" {
		opts.Name = "token-" + uniqueSafeName(t.Name())
	}
	if opts.Scope == "" {
		opts.Scope = "all"
	}

	resoureceAllRepos := len(opts.RepositoryIDs) == 0

	scope, err := auth_model.AccessTokenScope(opts.Scope).Normalize()
	require.NoError(t, err)

	accessToken := &auth_model.AccessToken{
		Name:             opts.Name,
		UID:              user.ID,
		Scope:            scope,
		ResourceAllRepos: resoureceAllRepos,
	}
	require.NoError(t, auth_model.NewAccessToken(t.Context(), accessToken))

	for _, repoID := range opts.RepositoryIDs {
		accessTokenResourceRepo := &auth_model.AccessTokenResourceRepo{
			TokenID: accessToken.ID,
			RepoID:  repoID,
		}
		require.NoError(t, db.Insert(t.Context(), accessTokenResourceRepo))
		t.Cleanup(func() {
			unittest.AssertSuccessfulDelete(t, &auth_model.AccessTokenResourceRepo{ID: accessTokenResourceRepo.ID})
		})
	}

	t.Cleanup(func() {
		unittest.AssertSuccessfulDelete(t, &auth_model.AccessToken{ID: accessToken.ID})
	})

	return accessToken
}
