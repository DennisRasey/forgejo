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
				// pass read / write access because the doer is admin
				data: newTestData(map[string]string{
					"packageOwner": "someuser",
				}, newSharedData().
					SetDoer().
					SetDoerAdmin(true),
				),
			},
			{
				// fail read / write access because the doer does not own the package
				data: newTestData(map[string]string{
					"packageOwner":           "someuser",
					"packageOwnerVisibility": "private",
				}, newSharedData().
					SetDoer(),
				),
				error: "user should have specific permission or be a site admin",
			},
		},
		sequenceFilter: []string{
			"APIAuthorization",
			signatureString,
		},
		fulfillNeeds: func(t *testing.T, data *testData) {
			t.Helper()
			data.shared.SetDoerDefault()
			if data.Get("packageOwner") == "doer" {
				data.Set("packageOwner", data.shared.DoerName())
			}
			data.SetDefault("packageOwner", data.shared.DoerName())
		},
		interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
			fixtureSetPackageOwner(t, permissions, data.Get("packageOwner"), data.Get("packageOwnerVisibility"))
		},
		staticArgs: 1,
		call: func(t *testing.T, ctx apiv1_permissions.Context, _ *testData, args []any) {
			mode := args[0].(perm.AccessMode)
			t.Logf("calling ReqPackageAccess(ctx, %s)", mode)
			apiv1_permissions.ReqPackageAccess(ctx, mode)
		},
	}
})
