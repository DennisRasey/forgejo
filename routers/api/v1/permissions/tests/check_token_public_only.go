// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"fmt"
	"strings"
	"testing"

	auth_model "forgejo.org/models/auth"
	org_model "forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

const (
	categoryActivityPub  = "activitypub"
	categoryAdmin        = "admin"
	categoryNotification = "notification"
	categoryOrganization = "organization"
	categoryPackage      = "package"
	categoryIssue        = "issue"
	categoryRepository   = "repository"
	categoryUser         = "user"
)

var categoryStringToCategory = map[string]auth_model.AccessTokenScopeCategory{
	categoryActivityPub:  auth_model.AccessTokenScopeCategoryActivityPub,
	categoryAdmin:        auth_model.AccessTokenScopeCategoryAdmin,
	categoryNotification: auth_model.AccessTokenScopeCategoryNotification,
	categoryOrganization: auth_model.AccessTokenScopeCategoryOrganization,
	categoryPackage:      auth_model.AccessTokenScopeCategoryPackage,
	categoryIssue:        auth_model.AccessTokenScopeCategoryIssue,
	categoryRepository:   auth_model.AccessTokenScopeCategoryRepository,
	categoryUser:         auth_model.AccessTokenScopeCategoryUser,
}

var _ = registerFunctionTestWithCall(apiv1_permissions.CheckTokenPublicOnly, functionTest{
	testCases: []*testCase{
		{
			// pass if public only is not set
			data: newTestData(map[string]string{}, newSharedData()),
		},
		{
			// pass if public only is set and a public repository is present
			data: newTestData(map[string]string{}, newSharedData().
				SetRepository().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// pass if public only is set and a public repository is present
			// and the token has a repository scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryRepository,
			}, newSharedData().
				SetRepository().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a private repository is present
			// and the token has a repository scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryRepository,
			}, newSharedData().
				SetRepository().
				SetRepositoryPrivate(true).
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public repos",
		},
		{
			// pass if public only is set and a public repository with issues is present
			// and the token has an issue scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryIssue,
			}, newSharedData().
				SetRepository().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a private repository with issues is present
			// and the token has an issue scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryIssue,
			}, newSharedData().
				SetRepository().
				SetRepositoryPrivate(true).
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public issues",
		},
		{
			// pass if public only is set and a public repository is present
			// and the token has a notification scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryNotification,
			}, newSharedData().
				SetRepository().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a private repository is present
			// and the token has a notification scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryNotification,
			}, newSharedData().
				SetRepository().
				SetRepositoryPrivate(true).
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public notifications",
		},
		{
			// pass if public only is set and a context user is present
			// and the token has a user scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryUser,
				"user":                    "someuser",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a private context user is present
			// and the token has a user scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryUser,
				"user":                    "username",
				"userVisibility":          "private",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public users",
		},
		{
			// pass if public only is set and a context user is present
			// and the token has an ActivityPub scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryActivityPub,
				"user":                    "someuser",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a private context user is present
			// and the token has an ActivityPub scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryActivityPub,
				"user":                    "username",
				"userVisibility":          "private",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public activitypub",
		},
		{
			// pass if public only is set and a context org is present
			// and the token has an organization scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryOrganization,
				"org":                     "orgname",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a private context org is present
			// and the token has an organization scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryOrganization,
				"org":                     "orgname",
				"orgVisibility":           "private",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public orgs",
		},
		{
			// pass if public only is set and a context user is present
			// but really is an organization
			// and the token has an organization scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryOrganization,
				"org":                     "orgname",
				"orgAsUser":               "true",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a context user is present
			// but really is a private organization
			// and the token has an organization scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryOrganization,
				"org":                     "orgname",
				"orgVisibility":           "private",
				"orgAsUser":               "true",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public orgs",
		},
		{
			// pass if public only is set and a context package is present
			// and the token has a package scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryPackage,
				"packageOwner":            "someuser",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
		},
		{
			// fail if public only is set and a context package is present
			// and the owner is a private user
			// and the token has a package scope
			data: newTestData(map[string]string{
				"requiredScopeCategories": categoryPackage,
				"packageOwner":            "username",
				"packageOwnerVisibility":  "private",
			}, newSharedData().
				SetDoerScope(fmt.Sprintf("%s", auth_model.AccessTokenScopePublicOnly)),
			),
			error: "token scope is limited to public packages",
		},
	},
	sequenceFilter: []string{
		"APIAuthorization",
		"CheckTokenPublicOnly",
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		data.shared.SetDoerDefault()
		data.shared.SetRepositoryDefault()
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.Has("user") {
			fixtureCreateUser(t, &user_model.User{Name: data.Get("user"), Visibility: stringToVisibility(data.Get("userVisibility"))})
		}
		if data.Has("org") {
			fixtureCreateOrg(t, &org_model.Organization{Name: data.Get("org"), Visibility: stringToVisibility(data.Get("orgVisibility"))}, &user_model.User{Name: data.shared.DoerName()})
		}
		if data.Has("packageOwner") {
			fixtureCreateUser(t, &user_model.User{Name: data.Get("packageOwner"), Visibility: stringToVisibility(data.Get("packageOwnerVisibility"))})
		}
		if data.Has("requiredScopeCategories") {
			var categories []auth_model.AccessTokenScopeCategory
			for categoryString := range strings.SplitSeq(data.Get("requiredScopeCategories"), ",") {
				categories = append(categories, categoryStringToCategory[categoryString])
			}
			permissions.SetRequiredScopeCategories(categories)
		}
		fixtureSetRepository(t, permissions, data.shared.RepositoryName(), data.shared.RepositoryInit(), data.shared.RepositoryPrivate(), data.shared.RepositoryArchived())
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		t.Helper()
		var user *user_model.User
		if data.Has("user") {
			user = fixtureGetUser(t, data.Get("user"))
		}
		var org *org_model.Organization
		if data.Has("org") {
			if data.Has("orgAsUser") {
				user = fixtureGetUser(t, data.Get("org"))
			} else {
				org = fixtureGetOrg(t, data.Get("org"))
			}
		}
		var packageOwner *user_model.User
		if data.Has("packageOwner") {
			packageOwner = fixtureGetUser(t, data.Get("packageOwner"))
		}
		t.Logf("calling CheckTokenPublicOnly(ctx, %+v, %+v, %+v)", user, org, packageOwner)
		apiv1_permissions.CheckTokenPublicOnly(ctx, user, org.AsUser(), packageOwner)
	},
})
