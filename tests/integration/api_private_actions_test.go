// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/url"
	"testing"

	"forgejo.org/models/actions"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/private"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateActionsRunnerToken(t *testing.T) {
	testCases := []struct {
		name          string
		scope         string
		expectedOwner optional.Option[int64]
		expectedRepo  optional.Option[int64]
		expectedError string
	}{
		{
			name:          "instance scope",
			scope:         "",
			expectedOwner: optional.None[int64](),
			expectedRepo:  optional.None[int64](),
		},

		{
			name:          "user scope",
			scope:         "user2",
			expectedOwner: optional.Some[int64](2),
			expectedRepo:  optional.None[int64](),
		},

		{
			name:          "organization scope",
			scope:         "org3",
			expectedOwner: optional.Some[int64](3),
			expectedRepo:  optional.None[int64](),
		},
		{
			name:          "unknown user",
			scope:         "does-not-exist",
			expectedError: "user does not exist",
		},
		{
			name:          "repository scope",
			scope:         "user2/test_workflows",
			expectedOwner: optional.None[int64](),
			expectedRepo:  optional.Some[int64](62),
		},
		{
			name:          "empty repository",
			scope:         "user2/",
			expectedError: "repository does not exist",
		},
		{
			name:          "unknown repository",
			scope:         "user2/does-not-exist",
			expectedError: "repository does not exist",
		},
		{
			name:          "owner mismatch",
			scope:         "org3/test_workflows",
			expectedError: "repository does not exist",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			onApplicationRun(t, func(*testing.T, *url.URL) {
				text, extra := private.GenerateActionsRunnerToken(t.Context(), testCase.scope)

				if testCase.expectedError == "" {
					require.NoError(t, extra.Error)

					newToken := unittest.AssertExistsAndLoadBean(t,
						&actions.ActionRunnerToken{OwnerID: testCase.expectedOwner, RepoID: testCase.expectedRepo})

					assert.Equal(t, newToken.Token, text.Text)
				} else {
					assert.ErrorContains(t, extra.Error, testCase.expectedError)
				}
			})
		})
	}
}
