// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.APIAuthorization, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{}, newSharedData().
				SetAnonymous(true),
			),
		},
		{
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer(),
			),
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.shared.SetDoerDefault()
		if data.shared.DoerActions() {
			data.shared.SetRepositoryDefault()
		}
		data.shared.SetDoerCanCreateOrganizationDefault(true)
		data.shared.SetDoerScopeDefault("read:repository")
		data.shared.SetTokenLevelDefault("read")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.shared.HasRepositoryName() && data.shared.DoerActions() {
			fixtureSetRepository(t, permissions, data.shared.RepositoryName(), data.shared.RepositoryInit(), data.shared.RepositoryPrivate(), data.shared.RepositoryArchived())
		}
		fixtureSetDoer(t, permissions, data)
	},
})
