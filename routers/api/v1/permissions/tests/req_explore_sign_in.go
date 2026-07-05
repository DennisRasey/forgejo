// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	"forgejo.org/modules/setting"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.ReqExploreSignIn, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"doer": "regularuser",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer":                      "anonymous",
				"Service.RequireSignInView": "true",
			}),
			error: "you must be signed in",
		},
		{
			data: newTestData(map[string]string{
				"doer":                              "anonymous",
				"Service.Explore.RequireSigninView": "true",
			}),
			error: "you must be signed in",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("doer", "regularuser")
	},
	protectSettingsBool: []*bool{
		&setting.Service.RequireSignInView,
		&setting.Service.Explore.RequireSigninView,
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureSetDoer(t, permissions, data)
		setting.Service.RequireSignInView = data.Get("Service.RequireSignInView") == "true"
		setting.Service.Explore.RequireSigninView = data.Get("Service.Explore.RequireSigninView") == "true"
	},
})
