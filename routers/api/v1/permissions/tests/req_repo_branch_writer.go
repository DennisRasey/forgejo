// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"strings"
	"testing"

	apiv1_permissions "forgejo.org/routers/api/v1/permissions"

	"github.com/stretchr/testify/require"
)

var _ = registerFunctionTestWithCall(apiv1_permissions.ReqRepoBranchWriter, functionTest{
	testCases: []*testCase{
		{
			// pass because the doer is the owner of the repository
			data: newTestData(map[string]string{
				"pullRequestAuthor": "userowner",
				"pullRequestBranch": "ReqRepoBranchWriter",
				"pullRequest":       "ReqRepoBranchWriter",
			}, newSharedData().
				SetDoer().SetDoerName("userowner").
				SetRepository().SetRepositoryName("userowner/repositorypublic").
				SetRepositoryInit(true),
			),
		},
		{
			// fail because the doer has no write permissions on the repository
			data: newTestData(map[string]string{
				"pullRequestAuthor": "userowner",
				"pullRequestBranch": "ReqRepoBranchWriter",
				"pullRequest":       "ReqRepoBranchWriter",
			}, newSharedData().
				SetDoer().
				SetRepositoryName("userowner/repositorypublic").
				SetRepositoryInit(true),
			),
			error: "user should have a permission to write to this branch",
		},
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		require.True(t, data.Has("pullRequestBranch"))
		fixtureCreateBranch(t, permissions, data.Get("pullRequestBranch"))
		require.True(t, data.Has("pullRequestAuthor"))
		require.True(t, data.Has("pullRequest"))
		fixtureCreatePullRequest(t, permissions, data.Get("pullRequest"), data.Get("pullRequestAuthor"), data.Get("pullRequestBranch"))
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		owner, _, found := strings.Cut(data.shared.RepositoryName(), "/")
		require.True(t, found)
		data.shared.SetDoer()
		data.shared.SetDoerName(owner)
		data.shared.SetRepositoryInitDefault(true)
		data.SetDefault("pullRequestAuthor", owner)
		data.SetDefault("pullRequestBranch", "ReqRepoBranchWriter")
		data.SetDefault("pullRequest", "ReqRepoBranchWriter")
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		branch := data.Get("pullRequestBranch")
		t.Logf("calling ReqRepoBranchWriter(ctx, %s)", branch)
		apiv1_permissions.ReqRepoBranchWriter(ctx, branch)
	},
})
