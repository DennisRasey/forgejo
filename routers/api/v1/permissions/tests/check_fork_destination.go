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
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *fixtureData) {
		require.True(t, data.Has("forkOrg"))
		if data.Get("forkOrg") == "unknownOrg" {
			return
		}
		require.True(t, data.Has("forkOrgOwner"))
		name := data.Get("forkOrg")
		owner := data.Get("forkOrgOwner")
		org := fixtureCreateOrg(t, &org_model.Organization{Name: name}, &user_model.User{Name: owner})

		if data.Has("team") {
			fixtureCreateTeam(t, org, data.Get("doer"), &forgery.CreateTeamOptions{
				Name:             data.Get("team"),
				CanCreateOrgRepo: data.Get("teamCanCreateOrgRepo") != "false",

				Mode: perm.AccessModeWrite,
			})
		}
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *fixtureData, _ []any) {
		forkOrg := data.Get("forkOrg")
		t.Logf("calling CheckForkDestination(ctx, %s)", forkOrg)
		apiv1_permissions.CheckForkDestination(ctx, &forkOrg)
	},
	fixtures: []*fixtureType{
		{
			data: newFixtureData(map[string]string{
				"doer":         "regularorgowner",
				"repository":   "userowner/repositorypublic",
				"forkOrg":      "regularorg1",
				"forkOrgOwner": "regularorgowner",
			}),
		},
		{
			data: newFixtureData(map[string]string{
				"doer":                 "regularuser",
				"repository":           "regularuser/repositorypublic",
				"forkOrg":              "regularorg1",
				"forkOrgOwner":         "regularorgowner",
				"team":                 "team1",
				"teamCanCreateOrgRepo": "true",
			}),
		},
		{
			data: newFixtureData(map[string]string{
				"doer":                 "regularuser",
				"repository":           "regularuser/repositorypublic",
				"forkOrg":              "regularorg1",
				"forkOrgOwner":         "regularorgowner",
				"team":                 "team1",
				"teamCanCreateOrgRepo": "false",
			}),
			error: "User is not allowed to create repos in Organisation",
		},
		{
			data: newFixtureData(map[string]string{
				"doer":         "doerregular",
				"repository":   "userowner/repositorypublic",
				"forkOrg":      "regularorg2",
				"forkOrgOwner": "regularorgowner",
			}),
			error: "User is no Member of Organisation 'regularorg2'",
		},
		{
			data: newFixtureData(map[string]string{
				"doer":       "regularorgowner",
				"repository": "userowner/repositorypublic",
				"forkOrg":    "unknownOrg",
			}),
			error: "org does not exist",
		},
	},
})
