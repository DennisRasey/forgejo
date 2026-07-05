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
			data: newTestData(map[string]string{
				"org":  "ReqTeamMembership",
				"team": org_model.OwnerTeamName,
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer": "doeradmin",
				"org":  "ReqTeamMembership",
				"team": org_model.OwnerTeamName,
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer":     "regularuser",
				"orgOwner": "orgOwner",
				"org":      "ReqTeamMembership",
				"teams":    "team1:regularuser",
				"team":     "team1",
			}),
		},
		{
			data: newTestData(map[string]string{
				"doer":     "regularuser",
				"orgOwner": "orgOwner",
				"org":      "ReqTeamMembership",
				"teams":    "team1:regularuser,team2:otheruser",
				"team":     "team2",
			}),
			error: "Must be a team member",
		},
		{
			data: newTestData(map[string]string{
				"doer":     "regularuser",
				"orgOwner": "orgOwner",
				"org":      "ReqTeamMembership",
				"teams":    "team2:otheruser",
				"team":     "team2",
			}),
			error: "Not Found",
		},
		{
			data: newTestData(map[string]string{
				"org": "ReqTeamMembership",
			}),
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
		orgOwner := data.Get("doer")
		if data.Has("orgOwner") {
			orgOwner = data.Get("orgOwner")
		}
		var org *org_model.Organization
		if data.Has("org") {
			fixtureCreateUser(t, &user_model.User{Name: orgOwner})
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
