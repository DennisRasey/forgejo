// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.ReqSelfOrAdmin, functionTest{
	testCases: []*testCase{
		{
			// pass because the doer is an admin user
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerAdmin(true)),
		},
		{
			// pass because the context user "someuser" is the same as the doer
			data: newTestData(map[string]string{
				"user": "someuser",
			}, newSharedData().
				SetDoer().
				SetDoerName("someuser"),
			),
		},
		{
			// fail because the doer is neither an admin nor is it equal to
			// the context user
			data: newTestData(map[string]string{
				"user": "otheruser",
			}, newSharedData().
				SetDoer(),
			),
			error: "doer should be the site admin or be same as the contextUser",
		},
	},
	sequenceFilter: []string{
		"APIAuthorization",
		"ReqSelfOrAdmin",
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		if !data.shared.HasDoerName() {
			data.shared.SetDoer()
			data.shared.SetDoerAdmin(true)
		}
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.Has("user") {
			name := data.Get("user")
			fixtureCreateUser(t, &user_model.User{Name: name})
			permissions.SetUser(fixtureGetUser(t, name))
		}
	},
})
