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
			data: newTestData(map[string]string{
				"repository": "userowner/repositorypublic",
				"doer":       "doeradmin",
			}),
		},
		{
			data: newTestData(map[string]string{
				"repository": "userowner/repositorypublic",
				"doer":       "userowner",
			}),
		},
		{
			data: newTestData(map[string]string{
				"repository": "userowner/repositorypublic",
				"doer":       "regularuser",
			}),
			error: "user should be an owner or a collaborator with admin write of a repository",
		},
	}
	for _, unitType := range unitTypes {
		unit := unitsTypeToString(unitType)
		fixtures = append(fixtures, &testCase{
			data: newTestData(map[string]string{
				"repository":    "userowner/repositorypublic",
				"doer":          "doeradmin",
				"disable-units": unit,
			}),
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
			fixtureDisableUnits(t, permissions, data)
		},
		fulfillNeeds: func(t *testing.T, data *testData) {
			t.Helper()
			data.Set("doer", "doeradmin")
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
