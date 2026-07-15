// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	"forgejo.org/models/unit"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.MustEnableIssues, functionTest{
	testCases: []*testCase{
		{
			// pass if a repository with issues unit set is present
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetRepository(),
			),
		},
		{
			// fail if a repository is present but the issues unit is disabled
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetRepository().
				SetRepositoryDisabledUnits([]unit.Type{unit.TypeIssues}),
			),
			error: "Not Found",
		},
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureDisableUnits(t, permissions, data.shared.RepositoryDisabledUnits())
	},
})
