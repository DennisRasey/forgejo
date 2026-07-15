// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	unit_model "forgejo.org/models/unit"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTestBuilder([]string{"ReqAdmin ", "ReqAdmin"}, func(t *testing.T, signatureString string, signature []any) {
	t.Helper()
	unitTypes := signature[1].([]unit_model.Type)
	fixtures := []*testCase{
		{
			// pass if the doer is admin
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerAdmin(true),
			),
		},
		{
			// pass if the doer is the owner of the repository
			data: newTestData(map[string]string{}, newSharedData().
				SetRepository().SetRepositoryName("userowner/repositorypublic").
				SetDoer().SetDoerName("userowner"),
			),
		},
		{
			// fail if the doer is neither admin nor the owner of the repository
			data: newTestData(map[string]string{}, newSharedData().
				SetRepository().
				SetDoer(),
			),
			error: "user should be an owner or a collaborator with admin write of a repository",
		},
	}
	for _, unitType := range unitTypes {
		fixtures = append(fixtures, &testCase{
			data: newTestData(map[string]string{}, newSharedData().
				SetRepositoryName("userowner/repositorypublic").
				SetRepositoryDisabledUnits([]unit_model.Type{unitType}).
				SetDoerName("root").
				SetDoerAdmin(true),
			),
			error: "Not Found",
		})
	}
	signatureStringToFunctionTest[signatureString] = functionTest{
		sequenceFilter: []string{
			"APIAuthorization",
			"RepoAccess",
			signatureString,
		},
		interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
			fixtureDisableUnits(t, permissions, data.shared.RepositoryDisabledUnits())
		},
		fulfillNeeds: func(t *testing.T, data *testData) {
			t.Helper()
			data.shared.SetDoer()
			data.shared.SetDoerAdmin(true)
		},
		testCases:  fixtures,
		staticArgs: 1,
		call: func(t *testing.T, ctx apiv1_permissions.Context, _ *testData, args []any) {
			unitTypes := args[0].([]unit_model.Type)
			t.Logf("calling ReqAdmin(ctx, %+v)", unitTypes)
			apiv1_permissions.ReqAdmin(ctx, unitTypes)
		},
	}
})
