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
			data: newTestData(map[string]string{
				"doer": "doeradmin",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer": "regularuser",
				"user": "regularuser",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer": "regularuser",
				"user": "otheruser",
			}),
			error: "doer should be the site admin or be same as the contextUser",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("doer", "doeradmin")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.Has("user") && data.Get("user") != "anonymous" {
			name := data.Get("user")
			user := permissions.User()
			if user == nil {
				fixtureCreateUser(t, &user_model.User{Name: name})
				permissions.SetUser(fixtureGetUser(t, name))
			}
		}
	},
})
