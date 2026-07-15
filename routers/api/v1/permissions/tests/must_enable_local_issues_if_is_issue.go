// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	unit_model "forgejo.org/models/unit"
	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"

	"github.com/stretchr/testify/require"
)

var _ = registerFunctionTestWithCall(apiv1_permissions.MustEnableLocalIssuesIfIsIssue, functionTest{
	testCases: []*testCase{
		{
			// pass if a repository with issues unit is present
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetRepository(),
			),
		},
		{
			// fail if an issue exists in a repository with issues unit disabled
			data: newTestData(map[string]string{
				"issue":       "issue5000",
				"issueAuthor": "issueAuthor",
			}, newSharedData().
				SetDoer().
				SetRepository().
				SetRepositoryDisabledUnits([]unit_model.Type{unit_model.TypeIssues}),
			),
			error: "Not Found",
		},
		{
			// pass if a pull request exists in a repository with issues unit disabled
			data: newTestData(map[string]string{
				"pullRequestAuthor": "userowner",
				"pullRequestBranch": "MustEnableLocalIssuesIfIsIssue",
				"pullRequest":       "MustEnableLocalIssuesIfIsIssue",
				"issue":             "MustEnableLocalIssuesIfIsIssue",
			}, newSharedData().
				SetDoer().
				SetRepository().SetRepositoryName("userowner/repositorypublic").
				SetRepositoryDisabledUnits([]unit_model.Type{unit_model.TypeIssues}).
				SetRepositoryInit(true),
			),
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("issue", "issueOne")
		data.SetDefault("issueAuthor", "issueAuthor")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureDisableUnits(t, permissions, data.shared.RepositoryDisabledUnits())
		if data.Has("pullRequest") {
			require.True(t, data.Has("pullRequestBranch"))
			fixtureCreateBranch(t, permissions, data.Get("pullRequestBranch"))
			require.True(t, data.Has("pullRequestAuthor"))
			require.True(t, data.Has("pullRequest"))
			fixtureCreatePullRequest(t, permissions, data.Get("pullRequest"), data.Get("pullRequestAuthor"), data.Get("pullRequestBranch"))
			require.Equal(t, data.Get("issue"), data.Get("pullRequest"))
		} else if data.Has("issue") {
			issueAuthor := fixtureCreateUser(t, &user_model.User{Name: data.Get("issueAuthor")})
			fixtureSetIssue(t, permissions, data.Get("issue"), issueAuthor.Name)
		}
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		t.Helper()
		var index int64
		if data.Has("issue") {
			index = fixtureGetIssue(t, data.Get("issue")).Index
		}
		t.Logf("calling MustEnableLocalIssuesIfIsIssue(ctx, %d)", index)
		apiv1_permissions.MustEnableLocalIssuesIfIsIssue(ctx, index)
	},
})
