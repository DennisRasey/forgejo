// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.MustAllowPulls, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"doer":            "doerregular",
				"repository":      "userowner/repositorypublic",
				"repository-init": "true",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer":            "doerregular",
				"repository":      "userowner/repositorypublic",
				"repository-init": "true",
				"disable-units":   "repo.pulls",
			}),
			error: "Not Found",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.Set("repository-init", "true")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureDisableUnits(t, permissions, data)
	},
})
