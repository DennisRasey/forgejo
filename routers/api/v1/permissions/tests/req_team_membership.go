// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	org_model "forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"

	"github.com/stretchr/testify/require"
)

var _ = registerFunctionTest(apiv1_permissions.ReqTeamMembership, functionTest{
	testCases: []*testCase{
		{
			// pass because the doer is the owner of the org and therefore member of the owner team
			data: newTestData(map[string]string{
				"org":  "ReqTeamMembership",
				"team": org_model.OwnerTeamName,
			}, newSharedData().
				SetDoer(),
			),
		},
		{
			// pass because the doer is admin although it is not a member of any team
			data: newTestData(map[string]string{
				"orgOwner": "orgOwner",
				"org":      "ReqTeamMembership",
				"team":     org_model.OwnerTeamName,
			}, newSharedData().
				SetDoer().
				SetDoerAdmin(true),
			),
		},
		{
			// pass because the doer is a member of the team1 in the org
			data: newTestData(map[string]string{
				"orgOwner": "orgOwner",
				"org":      "ReqTeamMembership",
				"teams":    "team1:someuser",
				"team":     "team1",
			}, newSharedData().
				SetDoer().SetDoerName("someuser"),
			),
		},
		{
			// fail because the doer is not a member of team2
			// the doer is a member of team1 in the org, but it is not the
			// team set in the context
			data: newTestData(map[string]string{
				"orgOwner": "orgOwner",
				"org":      "ReqTeamMembership",
				"teams":    "team1:someuser,team2:otheruser",
				"team":     "team2",
			}, newSharedData().
				SetDoer().SetDoerName("someuser"),
			),
			error: "Must be a team member",
		},
		{
			// fail because the doer is not a member of the context team
			// team2
			data: newTestData(map[string]string{
				"orgOwner": "orgOwner",
				"org":      "ReqTeamMembership",
				"teams":    "team2:otheruser",
				"team":     "team2",
			}, newSharedData().
				SetDoer(),
			),
			error: "Not Found",
		},
		{
			// fail because team is not set in the context
			data: newTestData(map[string]string{
				"org": "ReqTeamMembership",
			}, newSharedData()),
			error: "reqTeamMembership: unprepared context",
		},
	},
	sequenceFilter: []string{
		"APIAuthorization",
		"TokenRequiresScopes",
		"ReqTeamMembership",
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("org", "ReqTeamMembership")
		data.SetDefault("team", org_model.OwnerTeamName)
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		orgOwner := data.shared.DoerName()
		if data.Has("orgOwner") {
			orgOwner = data.Get("orgOwner")
		}
		var org *org_model.Organization
		if data.Has("org") {
			fixtureCreateUser(t, &user_model.User{Name: orgOwner, AllowCreateOrganization: true})
			org = fixtureCreateOrg(t, &org_model.Organization{Name: data.Get("org")}, &user_model.User{Name: orgOwner})
		}

		if data.Has("teams") {
			fixtureCreateTeams(t, org, data.Get("teams"))
		}

		if data.Has("team") {
			team, err := org_model.GetTeam(t.Context(), org.ID, data.Get("team"))
			require.NoError(t, err)
			permissions.SetTeam(team)
		}
	},
})
