// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	"forgejo.org/modules/setting"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.ReqWebhooksEnabled, functionTest{
	testCases: []*testCase{
		{
			// pass because the DisableWebhooks setting is false by default
			data: newTestData(map[string]string{}, newSharedData()),
		},
		{
			// fail because the DisableWebhooks setting is set to true
			data: newTestData(map[string]string{
				"DisableWebhooks": "true",
			}, newSharedData()),
			error: "webhooks disabled by administrator",
		},
	},
	protectSettingsBool: []*bool{
		&setting.DisableWebhooks,
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		setting.DisableWebhooks = data.Get("DisableWebhooks") == "true"
	},
})
