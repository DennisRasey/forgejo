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

var _ = registerFunctionTest(apiv1_permissions.ReqOrgOwnership, functionTest{
	testCases: []*testCase{
		{
			// pass because the doer is the owner of the org
			data: newTestData(map[string]string{
				"org":    "ReqOrgOwnershipOrg",
				"setOrg": "true",
			}, newSharedData().
				SetDoer(),
			),
		},
		{
			// pass because the doer is admin even though it is not the
			// owner of the org
			data: newTestData(map[string]string{
				"org":      "ReqOrgOwnershipOrg",
				"orgOwner": "ReqOrgOwnershipOrgOwner",
				"setOrg":   "true",
			}, newSharedData().
				SetDoer().
				SetDoerAdmin(true),
			),
		},
		{
			// fail because the doer is not the owner of the org
			data: newTestData(map[string]string{
				"org":      "ReqOrgOwnershipOrg",
				"orgOwner": "ReqOrgOwnershipOrgOwner",
				"setOrg":   "true",
			}, newSharedData().
				SetDoer(),
			),
			error: "Must be an organization owner",
		},
		{
			// pass because the doer is in the context team that owns the
			// org
			data: newTestData(map[string]string{
				"org":     "ReqOrgOwnershipOrg",
				"setTeam": "true",
			}, newSharedData().
				SetDoer(),
			),
		},
		{
			// fail because the doer is in the context team that does
			// not own the org
			data: newTestData(map[string]string{
				"org":      "ReqOrgOwnershipOrg",
				"orgOwner": "ReqOrgOwnershipOrgOwner",
				"setTeam":  "true",
			}, newSharedData().
				SetDoer(),
			),
			error: "Not Found",
		},
		{
			// fail because the context org is not set
			data: newTestData(map[string]string{
				"setOrg": "true",
			}, newSharedData()),
			error: "reqOrgOwnership: unprepared context",
		},
	},
	sequenceFilter: []string{
		"APIAuthorization",
		"TokenRequiresScopes",
		"ReqOrgOwnership",
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("org", "ReqOrgOwnershipOrg")
		data.SetDefault("setOrg", "true")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		orgOwner := data.shared.DoerName()
		if data.Has("orgOwner") {
			orgOwner = data.Get("orgOwner")
		}
		var org *org_model.Organization
		if data.Has("org") {
			fixtureCreateUser(t, &user_model.User{Name: orgOwner})
			org = fixtureCreateOrg(t, &org_model.Organization{Name: data.Get("org")}, &user_model.User{Name: orgOwner})
		}

		if data.Get("setOrg") == "true" {
			permissions.SetOrganization(org)
		}

		if data.Get("setTeam") == "true" {
			team, err := org_model.GetTeam(t.Context(), org.ID, org_model.OwnerTeamName)
			require.NoError(t, err)
			permissions.SetTeam(team)
		}
	},
})
