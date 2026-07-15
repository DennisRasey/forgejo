// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	org_model "forgejo.org/models/organization"
	"forgejo.org/models/perm"
	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/require"
)

var _ = registerFunctionTestWithCall(apiv1_permissions.CheckForkDestination, functionTest{
	testCases: []*testCase{
		{
			// the doer creates the fork in an organization where it is the
			// owner
			data: newTestData(map[string]string{
				"forkOrg":      "someorg1",
				"forkOrgOwner": "someorgowner",
			}, newSharedData().
				SetDoer().SetDoerName("someorgowner").
				SetRepository(),
			),
		},
		{
			// the doer creates the fork in an organization where it
			// belongs to a team that is allowed to create repos
			data: newTestData(map[string]string{
				"forkOrg":              "someorg1",
				"forkOrgOwner":         "someorgowner",
				"team":                 "team1",
				"teamCanCreateOrgRepo": "true",
			}, newSharedData().
				SetDoer().
				SetRepository(),
			),
		},
		{
			// the doer is not allowed to create a fork in a organization
			// where it belongs to a team that is not allowed to create repos
			data: newTestData(map[string]string{
				"forkOrg":              "someorg1",
				"forkOrgOwner":         "someorgowner",
				"team":                 "team1",
				"teamCanCreateOrgRepo": "false",
			}, newSharedData().
				SetDoer().
				SetRepository(),
			),
			error: "User is not allowed to create repos in Organisation",
		},
		{
			// the doer is not allowed to create a fork in an organization
			// where it is not a member of any team
			data: newTestData(map[string]string{
				"forkOrg":      "someorg2",
				"forkOrgOwner": "someorgowner",
			}, newSharedData().
				SetDoer().
				SetRepository(),
			),
			error: "User is no Member of Organisation 'someorg2'",
		},
		{
			// an attempt to create a fork in an unknown organization fails
			data: newTestData(map[string]string{
				"forkOrg": "unknownOrg",
			}, newSharedData().
				SetDoer().
				SetRepository(),
			),
			error: "org does not exist",
		},
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		require.True(t, data.Has("forkOrg"))
		if data.Get("forkOrg") == "unknownOrg" {
			return
		}
		require.True(t, data.Has("forkOrgOwner"))
		name := data.Get("forkOrg")
		owner := data.Get("forkOrgOwner")
		org := fixtureCreateOrg(t, &org_model.Organization{Name: name}, &user_model.User{Name: owner})

		if data.Has("team") {
			fixtureCreateTeam(t, org, data.shared.DoerName(), &forgery.CreateTeamOptions{
				Name:             data.Get("team"),
				CanCreateOrgRepo: data.Get("teamCanCreateOrgRepo") != "false",

				Mode: perm.AccessModeWrite,
			})
		}
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		forkOrg := data.Get("forkOrg")
		t.Logf("calling CheckForkDestination(ctx, %s)", forkOrg)
		apiv1_permissions.CheckForkDestination(ctx, &forkOrg)
	},
})
