// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.IndividualPermsChecker, functionTest{
	testCases: []*testCase{
		{
			// pass if a public context user exists
			data: newTestData(map[string]string{
				"user": "IndividualPermsChecker",
			}, newSharedData()),
		},
		{
			// fail if a private context user exists
			data: newTestData(map[string]string{
				"user":           "IndividualPermsCheckerOne",
				"userVisibility": "private",
			}, newSharedData()),
			error: "Visit Project",
		},
		{
			// fail if a limited context user exists
			data: newTestData(map[string]string{
				"user":           "IndividualPermsCheckerTwo",
				"userVisibility": "limited",
			}, newSharedData().SetAnonymous(true)),
			error: "Visit Project",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("user", data.shared.DoerName())
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.Has("user") {
			name := data.Get("user")
			visibility := data.Get("userVisibility")
			fixtureCreateUser(t, &user_model.User{Name: name, Visibility: stringToVisibility(visibility)})
			permissions.SetUser(fixtureGetUser(t, name))
		}
	},
})
