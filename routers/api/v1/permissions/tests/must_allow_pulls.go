// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	unit_model "forgejo.org/models/unit"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.MustAllowPulls, functionTest{
	testCases: []*testCase{
		{
			// pass if a repository with pull requests unit set is present
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetRepository().
				SetRepositoryInit(true),
			),
		},
		{
			// fail if a repository is present but the pull requests unit is disabled
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetRepository().
				SetRepositoryInit(true).
				SetRepositoryDisabledUnits([]unit_model.Type{unit_model.TypePullRequests}),
			),
			error: "Not Found",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.shared.SetRepositoryInit(true)
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		fixtureDisableUnits(t, permissions, data.shared.RepositoryDisabledUnits())
	},
})
