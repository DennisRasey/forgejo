// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	"forgejo.org/models/perm"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTestBuilder([]string{"ReqPackageAccess "}, func(_ *testing.T, signatureString string, signature []any) {
	signatureStringToFunctionTest[signatureString] = functionTest{
		testCases: []*testCase{
			{
				data: newTestData(map[string]string{
					"packageOwner": "doer",
					"doer":         "doeradmin",
				}),
			},
			{
				data: newTestData(map[string]string{
					"doer":         "userregular",
					"packageOwner": "userprivate",
				}),
				error: "user should have specific permission or be a site admin",
			},
		},
		sequenceFilter: []string{
			"APIAuthorization",
			signatureString,
		},
		fulfillNeeds: func(t *testing.T, data *testData) {
			t.Helper()
			data.SetDefault("doer", "doerregular")
			if data.Get("packageOwner") == "doer" {
				data.Set("packageOwner", data.Get("doer"))
			}
			data.SetDefault("packageOwner", data.Get("doer"))
		},
		interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
			fixtureSetPackageOwner(t, permissions, data)
		},
		staticArgs: 1,
		call: func(t *testing.T, ctx apiv1_permissions.Context, _ *testData, args []any) {
			mode := args[0].(perm.AccessMode)
			t.Logf("calling ReqPackageAccess(ctx, %s)", mode)
			apiv1_permissions.ReqPackageAccess(ctx, mode)
		},
	}
})
