// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"strings"
	"testing"

	org_model "forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"

	"github.com/stretchr/testify/require"
)

var _ = registerFunctionTestWithCall(apiv1_permissions.TokenRequiresRepoOwnerScope, functionTest{
	testCases: []*testCase{
		{
			data: newTestData(map[string]string{
				"owner": "doerregular",
				"scope": "read:user",
				"level": "read",
			}),
		},
		{
			data: newTestData(map[string]string{
				"owner": "doerregular",
				"scope": "read:user",
				"level": "write",
			}),
			error: "token does not have at least one of required scope(s): [write:user]",
		},
		{
			data: newTestData(map[string]string{
				"owner": "regularorg",
				"scope": "read:organization",
				"level": "read",
			}),
		},
		{
			data: newTestData(map[string]string{
				"owner": "regularorg",
				"scope": "read:organization",
				"level": "write",
			}),
			error: "token does not have at least one of required scope(s): [write:organization]",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		if !data.Has("owner") {
			if data.Has("repository") {
				owner, _, found := strings.Cut(data.Get("repository"), "/")
				require.True(t, found)
				data.Set("owner", owner)
			} else {
				data.Set("owner", "doerregular")
			}
		}
		data.SetDefault("level", "read")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		ownerName := data.Get("owner")
		if strings.Contains(ownerName, "org") {
			fixtureCreateOrg(t, &org_model.Organization{Name: ownerName}, &user_model.User{Name: "orgOwner" + ownerName})
			require.NotNil(t, fixtureGetUser(t, ownerName))
		} else {
			fixtureCreateUser(t, &user_model.User{Name: ownerName})
		}
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		t.Helper()
		owner := fixtureGetUser(t, data.Get("owner"))
		level := levelStringToLevel(data.Get("level"))
		t.Logf("calling TokenRequiresRepoOwnerScope(ctx, %+v, %v)", owner, level)
		apiv1_permissions.TokenRequiresRepoOwnerScope(ctx, owner, level)
	},
})
