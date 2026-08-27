// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.DoerCanCreateOrganization, functionTest{
	testCases: []*testCase{
		{
			// the doer can create an organization
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerCanCreateOrganization(true),
			),
		},
		{
			// the doer is admin
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerCanCreateOrganization(false).
				SetDoerAdmin(true),
			),
		},
		{
			// the doer cannot create an organization
			data: newTestData(map[string]string{}, newSharedData().
				SetDoer().
				SetDoerCanCreateOrganization(false),
			),
			error: "Create organization not allowed",
		},
	},
})
