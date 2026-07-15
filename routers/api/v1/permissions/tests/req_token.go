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
			// pass because the doer is authenticated with a token
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer(),
			),
		},
		{
			// pass because the Forgejo Actions doer is authenticated with a token
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().SetDoerName(user_model.ActionsUserName).
				SetDoerActions(true),
			),
		},
		{
			// fail because an anonymous visitor is not authenticated with a token
			data: newTestData(map[string]string{}, newSharedData().
				SetAnonymous(true),
			),
			error: "token is required",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.shared.SetDoerDefault()
	},
})
