// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package private

import (
	"testing"

	"forgejo.org/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScope(t *testing.T) {
	testCases := []struct {
		name          string
		scope         string
		expectedOwner int64
		expectedRepo  int64
		expectedError string
	}{
		{
			name:          "instance scope",
			scope:         "",
			expectedOwner: 0,
			expectedRepo:  0,
		},
		{
			name:          "user scope",
			scope:         "user2",
			expectedOwner: 2,
			expectedRepo:  0,
		},
		{
			name:          "organization scope",
			scope:         "org3",
			expectedOwner: 3,
			expectedRepo:  0,
		},
		{
			name:          "unknown user",
			scope:         "does-not-exist",
			expectedError: "user does not exist",
		},
		{
			name:          "repository scope",
			scope:         "user2/test_workflows",
			expectedOwner: 0,
			expectedRepo:  62,
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
			require.NoError(t, unittest.PrepareTestDatabase())

			owner, repo, err := ParseScope(t.Context(), testCase.scope)

			if testCase.expectedError == "" {
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedOwner, owner)
				assert.Equal(t, testCase.expectedRepo, repo)
			} else {
				require.ErrorContains(t, err, testCase.expectedError)
			}
		})
	}
}
