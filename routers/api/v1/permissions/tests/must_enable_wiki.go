// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.MustEnableWiki, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"doer":       "doerregular",
				"repository": "userowner/repositorypublic",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer":          "doerregular",
				"repository":    "userowner/repositorypublic",
				"disable-units": "repo.wiki",
			}),
			error: "Not Found",
		},
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureDisableUnits(t, permissions, data)
	},
})
