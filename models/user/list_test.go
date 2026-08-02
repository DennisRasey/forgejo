// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"testing"

	"forgejo.org/models/auth"
	"forgejo.org/models/unittest"

	"github.com/stretchr/testify/require"
)

func TestLoadTwoFactorStatus(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, tt := range []struct {
		name               string
		userIDs            []int64
		expectedTwoFactors []*auth.TwoFactor
	}{
		{
			"no users",
			[]int64{},
			[]*auth.TwoFactor{},
		},
		{
			"one user without 2FA",
			[]int64{1},
			[]*auth.TwoFactor{},
		},
		{
			"one user with 2FA",
			[]int64{24},
			[]*auth.TwoFactor{{
				ID:               1,
				UID:              24,
				Secret:           []byte("MrAed+7K+fKQKu1l3aU45oTDSWK/i5Ugtgk8CmORrKWTMwa2w97rniLU+h+2xq8ZF+16uuXGLzjWa0bOV5xg4NY6w5Ec/tkwQ5rEecOTvc/JZV5lrrlDi48B7Y5/lNcjAWBmH2nEUlM="),
				ScratchSalt:      "Qb5bq2DyR2",
				ScratchHash:      "068eb9b8746e0bcfe332fac4457693df1bda55800eb0f6894d14ebb736ae6a24e0fc8fc5333c19f57f81599788f0b8e51ec1",
				LastUsedPasscode: "",
				CreatedUnix:      1564253724,
				UpdatedUnix:      1564253724,
			}},
		},
		{
			"many users",
			[]int64{1, 2, 3, 4, 5, 10, 20, 21, 22, 23, 24},
			[]*auth.TwoFactor{{
				ID:               1,
				UID:              24,
				Secret:           []byte("MrAed+7K+fKQKu1l3aU45oTDSWK/i5Ugtgk8CmORrKWTMwa2w97rniLU+h+2xq8ZF+16uuXGLzjWa0bOV5xg4NY6w5Ec/tkwQ5rEecOTvc/JZV5lrrlDi48B7Y5/lNcjAWBmH2nEUlM="),
				ScratchSalt:      "Qb5bq2DyR2",
				ScratchHash:      "068eb9b8746e0bcfe332fac4457693df1bda55800eb0f6894d14ebb736ae6a24e0fc8fc5333c19f57f81599788f0b8e51ec1",
				LastUsedPasscode: "",
				CreatedUnix:      1564253724,
				UpdatedUnix:      1564253724,
			}},
		},
	} {
		users := make(UserList, 0, len(tt.userIDs))
		for _, userID := range tt.userIDs {
			users = append(users, &User{ID: userID})
		}

		expectedTwoFactors := make(map[int64]*auth.TwoFactor)
		for _, twoFactor := range tt.expectedTwoFactors {
			expectedTwoFactors[twoFactor.ID] = twoFactor
		}

		twoFactors, err := users.loadTwoFactorStatus(t.Context())
		require.NoError(t, err, "could not load two factor status in test '%s'", tt.name)

		require.Equal(t, expectedTwoFactors, twoFactors, "in test '%s'", tt.name)
	}
}
