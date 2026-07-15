// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	"forgejo.org/modules/setting"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.ReqBasicOrRevProxyAuth, functionTest{
	testCases: []*testCase{
		{
			// pass if the doer is authenticated via a proxy
			data: newTestData(map[string]string{
				"Service.EnableReverseProxyAuthAPI": "true",
			}, newSharedData().
				SetDoer().
				SetDoerAuthentication("proxy"),
			),
		},
		{
			// pass if the doer is authenticated using user / password (basic)
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerAuthentication("basic"),
			),
		},
		{
			// fail if the doer is authenticated using a token
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerAuthentication("token"),
			),
			error: "auth method not allowed",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.shared.SetDoerDefault()
		data.shared.SetDoerAuthenticationDefault("proxy")
		data.SetDefault("Service.EnableReverseProxyAuthAPI", "true")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureSetDoer(t, permissions, data)
		setting.Service.EnableReverseProxyAuthAPI = data.Get("Service.EnableReverseProxyAuthAPI") == "true"
	},
})
