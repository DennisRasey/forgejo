// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"

	"github.com/stretchr/testify/require"
)

var _ = registerFunctionTestWithCall(apiv1_permissions.MustEnableLocalIssuesIfIsIssue, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"doer":        "doerregular",
				"repository":  "userowner/repositorypublic",
				"issue":       "issue5000",
				"issueAuthor": "issueAuthor",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer":          "doerregular",
				"repository":    "userowner/repositorypublic",
				"issue":         "issue5000",
				"issueAuthor":   "issueAuthor",
				"disable-units": "repo.issues",
			}),
			error: "Not Found",
		},
		{ // does not fail because it is an issue instead of a pull request
			data: newTestData(map[string]string{
				"doer":              "userowner",
				"repository":        "userowner/repositorypublic",
				"repository-init":   "true",
				"pullRequestAuthor": "userowner",
				"pullRequestBranch": "MustEnableLocalIssuesIfIsIssue",
				"pullRequest":       "MustEnableLocalIssuesIfIsIssue",
				"issue":             "MustEnableLocalIssuesIfIsIssue",
				"disable-units":     "repo.issues",
			}),
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("issue", "issueOne")
		data.SetDefault("issueAuthor", "issueAuthor")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureDisableUnits(t, permissions, data)
		if data.Has("pullRequest") {
			require.True(t, data.Has("pullRequestBranch"))
			fixtureCreateBranch(t, permissions, data.Get("pullRequestBranch"))
			require.True(t, data.Has("pullRequestAuthor"))
			require.True(t, data.Has("pullRequest"))
			fixtureCreatePullRequest(t, permissions, data)
			require.Equal(t, data.Get("issue"), data.Get("pullRequest"))
		} else {
			fixtureCreateUser(t, &user_model.User{Name: data.Get("issueAuthor")})
			fixtureSetIssue(t, permissions, data)
		}
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		t.Helper()
		index := fixtureGetIssue(t, data).Index
		t.Logf("calling MustEnableLocalIssuesIfIsIssue(ctx, %d)", index)
		apiv1_permissions.MustEnableLocalIssuesIfIsIssue(ctx, index)
	},
})
