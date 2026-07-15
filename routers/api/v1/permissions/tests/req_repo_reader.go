// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	unit_model "forgejo.org/models/unit"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTestBuilder([]string{"ReqRepoReader "}, func(t *testing.T, signatureString string, signature []any) {
	t.Helper()
	unitType := signature[1].(unit_model.Type)
	signatureStringToFunctionTest[signatureString] = functionTest{
		testCases: []*testCase{
			{
				// pass because the doer can read the public repository
				data: newTestData(map[string]string{}, newSharedData().
					SetDoer().
					SetRepository(),
				),
			},
			{
				// fail because the repository unitType is disabled
				data: newTestData(map[string]string{}, newSharedData().
					SetDoer().
					SetRepository().
					SetRepositoryDisabledUnits([]unit_model.Type{unitType}),
				),
				error: "Not Found",
			},
			// This fixture is unreachable because this permissions function is always used after
			// a RepoAccess that enforces the same restriction for non admin users
			// {
			//	data: newTestData(map[string]string{}, newSharedData()),
			// 	error: "user should have specific read permission or be a repo admin or a site admin",
			// },
		},
		sequenceFilter: []string{
			"APIAuthorization",
			"RepoAccess",
			signatureString,
		},
		interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
			fixtureDisableUnits(t, permissions, data.shared.RepositoryDisabledUnits())
		},
		staticArgs: 1,
		call: func(t *testing.T, ctx apiv1_permissions.Context, _ *testData, args []any) {
			unitType := args[0].(unit_model.Type)
			t.Logf("calling ReqRepoReader(ctx, %s)", unitType)
			apiv1_permissions.ReqRepoReader(ctx, unitType)
		},
	}
})
