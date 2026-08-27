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
			// pass because the owner of the default repository is either an
			// org or user publicly readable
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerScope("read:user,read:organization").
				SetTokenLevel("read"),
			),
		},
		{
			// pass because the user "username" is readable by default
			data: newTestData(map[string]string{
				"user": "username",
			}, newSharedData().
				SetDoer().
				SetDoerScope("read:user").
				SetTokenLevel("read"),
			),
		},
		{
			// fail because the user "username" is not writable by the doer
			data: newTestData(map[string]string{
				"user": "username",
			}, newSharedData().
				SetDoer().
				SetDoerScope("read:user").
				SetTokenLevel("write"),
			),
			error: "token does not have at least one of required scope(s): [write:user]",
		},
		{
			// pass because the org "orgname" is readable by default
			data: newTestData(map[string]string{
				"org": "orgname",
			}, newSharedData().
				SetDoer().
				SetDoerScope("read:organization").
				SetTokenLevel("read"),
			),
		},
		{
			// fail because the org "orgname" is not writable by the doer
			data: newTestData(map[string]string{
				"org": "orgname",
			}, newSharedData().
				SetDoer().
				SetDoerScope("read:organization").
				SetTokenLevel("write"),
			),
			error: "token does not have at least one of required scope(s): [write:organization]",
		},
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.shared.SetTokenLevelDefault("read")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.Has("org") {
			orgName := data.Get("org")
			fixtureCreateOrg(t, &org_model.Organization{Name: orgName}, &user_model.User{Name: "orgOwner" + orgName, AllowCreateOrganization: true})
			require.NotNil(t, fixtureGetUser(t, orgName))
		} else if data.Has("user") {
			fixtureCreateUser(t, &user_model.User{Name: data.Get("user")})
		}
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		t.Helper()
		var ownerName string
		if data.Has("org") {
			ownerName = data.Get("org")
		} else if data.Has("user") {
			ownerName = data.Get("user")
		} else {
			var found bool
			ownerName, _, found = strings.Cut(data.shared.RepositoryName(), "/")
			require.True(t, found)
		}
		owner := fixtureGetUser(t, ownerName)
		level := levelStringToLevel(data.shared.TokenLevel())
		t.Logf("calling TokenRequiresRepoOwnerScope(ctx, %s, %v)", ownerName, level)
		apiv1_permissions.TokenRequiresRepoOwnerScope(ctx, owner, level)
	},
})
