// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"fmt"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTestBuilder([]string{"TokenRequiresScopes "}, func(t *testing.T, signatureString string, signature []any) {
	t.Helper()
	categories := signature[1].([]auth_model.AccessTokenScopeCategory)
	var scopes []string
	for _, category := range categories {
		var scope auth_model.AccessTokenScope
		switch category {
		case auth_model.AccessTokenScopeCategoryActivityPub:
			scope = auth_model.AccessTokenScopeReadActivityPub
		case auth_model.AccessTokenScopeCategoryAdmin:
			scope = auth_model.AccessTokenScopeReadAdmin
		case auth_model.AccessTokenScopeCategoryNotification:
			scope = auth_model.AccessTokenScopeReadNotification
		case auth_model.AccessTokenScopeCategoryOrganization:
			scope = auth_model.AccessTokenScopeReadOrganization
		case auth_model.AccessTokenScopeCategoryPackage:
			scope = auth_model.AccessTokenScopeReadPackage
		case auth_model.AccessTokenScopeCategoryIssue:
			scope = auth_model.AccessTokenScopeReadIssue
		case auth_model.AccessTokenScopeCategoryRepository:
			scope = auth_model.AccessTokenScopeReadRepository
		case auth_model.AccessTokenScopeCategoryUser:
			scope = auth_model.AccessTokenScopeReadUser
		default:
			panic(fmt.Errorf("unexpected category %v", category))
		}
		scopes = append(scopes, string(scope))
	}
	readscope := strings.Join(scopes, ",")
	t.Logf("%s scopes %s", signatureString, readscope)
	signatureStringToFunctionTest[signatureString] = functionTest{
		testCases: []*testCase{
			{
				data: newTestData(map[string]string{
					"doer":  "doerregular",
					"scope": readscope,
					"level": "read",
				}),
			},
			{
				data: newTestData(map[string]string{
					"doer":  "doerregular",
					"scope": readscope,
					"level": "write",
				}),
				error: "token does not have at least one of required scope(s)",
			},
			{
				data: newTestData(map[string]string{
					"doer":  "doerregular",
					"scope": "read:misc",
					"level": "read",
				}),
				error: "token does not have at least one of required scope(s)",
			},
		},
		fulfillNeeds: func(t *testing.T, data *testData) {
			t.Helper()
			data.SetDefault("repository", "userowner/repositorypublic")
			data.SetDefault("doer", "doerregular")
			if data.Has("scope") {
				scope := data.Get("scope")
				if !strings.Contains(scope, readscope) {
					writescope := strings.ReplaceAll(readscope, "read", "write")
					data.Set("scope", strings.Join([]string{scope, writescope}, ","))
				}
			} else {
				data.Set("scope", readscope)
			}

			data.SetDefault("level", "read")
		},
		interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
			fixtureSetRepository(t, permissions, data)
		},
		staticArgs: 1,
		call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, args []any) {
			level := levelStringToLevel(data.Get("level"))
			categories := args[0].([]auth_model.AccessTokenScopeCategory)
			t.Logf("calling TokenRequiresScopes(ctx, %v, %v)", categories, level)
			apiv1_permissions.TokenRequiresScopes(ctx, categories, level)
		},
	}
})
