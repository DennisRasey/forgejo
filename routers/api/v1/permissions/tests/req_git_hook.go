// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	"forgejo.org/modules/setting"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.ReqGitHook, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"doer": "doeradmin",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer":            "doeradmin",
				"DisableGitHooks": "true",
			}),
			error: "must be allowed to edit Git hooks",
		},
	},
	protectSettingsBool: []*bool{
		&setting.DisableGitHooks,
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("doer", "doeradmin")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		setting.DisableGitHooks = data.Get("DisableGitHooks") == "true"
	},
})
