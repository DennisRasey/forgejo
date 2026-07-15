// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"strings"
	"testing"

	unit_model "forgejo.org/models/unit"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"

	"github.com/stretchr/testify/require"
)

var _ = registerFunctionTestBuilder([]string{"ReqRepoWriter "}, func(t *testing.T, signatureString string, signature []any) {
	t.Helper()
	unitTypes := signature[1].([]unit_model.Type)
	scopes := unitsToScopes(unitTypes, "write")
	signatureStringToFunctionTest[signatureString] = functionTest{
		testCases: []*testCase{
			{
				// pass because the doer is the owner of the repository
				data: newTestData(map[string]string{}, newSharedData().
					SetDoer().SetDoerName("userowner").
					SetRepository().SetRepositoryName("userowner/repositorypublic").
					SetDoerScope(scopes),
				),
			},
			{
				// fail because the repository unitTypes are disabled
				data: newTestData(map[string]string{}, newSharedData().
					SetRepositoryDisabledUnits(unitTypes),
				),
				error: "Not Found",
			},
			{
				// fail because the doer is not the owner of the repository
				// and does not have write permission
				data: newTestData(map[string]string{}, newSharedData().
					SetDoer().
					SetDoerScope(scopes).
					SetRepositoryName("userowner/repositorypublic"),
				),
				error: "user should have a permission to write to a repo",
			},
		},
		sequenceFilter: []string{
			"APIAuthorization",
			"RepoAccess",
			signatureString,
		},
		interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
			fixtureDisableUnits(t, permissions, data.shared.RepositoryDisabledUnits())
		},
		fulfillNeeds: func(t *testing.T, data *testData) {
			t.Helper()
			if data.shared.HasRepositoryName() {
				owner, _, found := strings.Cut(data.shared.RepositoryName(), "/")
				require.True(t, found)
				data.shared.SetDoer()
				data.shared.SetDoerName(owner)
			} else {
				data.shared.SetRepositoryDefault()
				data.shared.SetRepositoryNameDefault("userowner/repositorypublic")
				data.shared.SetDoer()
				data.shared.SetDoerNameDefault("userowner")
			}
			data.shared.SetTokenLevelDefault("write")
		},
		staticArgs: 1,
		call: func(t *testing.T, ctx apiv1_permissions.Context, _ *testData, args []any) {
			unitType := args[0].([]unit_model.Type)
			t.Logf("calling ReqRepoWriter(ctx, %s)", unitType)
			apiv1_permissions.ReqRepoWriter(ctx, unitType)
		},
	}
})
