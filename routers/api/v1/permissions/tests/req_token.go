// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.ReqToken, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"doer": "doerregular",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer": user_model.ActionsUserName,
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer": "anonymous",
			}),
			error: "token is required",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("doer", "doerregular")
	},
})
