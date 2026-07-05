// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.APIAuthorization, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"doer": "anonymous",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer": "doerregular",
			}),
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("doer", "doerregular")
		if data.Get("doer") == user_model.ActionsUserName {
			data.SetDefault("repository", "userowner/repositorypublic")
		}
		data.SetDefault("scope", "read:repository")
		data.SetDefault("level", "read")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.Has("repository") && data.Get("doer") == user_model.ActionsUserName {
			fixtureSetRepository(t, permissions, data)
		}
		fixtureSetDoer(t, permissions, data)
	},
})
