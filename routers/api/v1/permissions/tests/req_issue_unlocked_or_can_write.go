// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	unit_model "forgejo.org/models/unit"
	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTestWithCall(apiv1_permissions.ReqIssueUnlockedOrCanWrite, functionTest{
	testCases: []*testCase{
		{
			// pass because the issue is not locked and in a public repository
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
			}, newSharedData().
				SetDoer().
				SetRepository(),
			),
		},
		{
			// fail because the issue is locked
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueLocked": "true",
			}, newSharedData().
				SetDoer().
				SetRepository(),
			),
			error: "You cannot change a locked issue",
		},
		{
			// fail because the issue unit is disabled
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
			}, newSharedData().
				SetDoer().
				SetRepository().
				SetRepositoryDisabledUnits([]unit_model.Type{unit_model.TypeIssues}),
			),
			error: "Not Found",
		},
		{
			// pass because the doer is the owner of the repository and has
			// write access to the issues unit and can change them when they
			// are locked
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueLocked": "true",
			}, newSharedData().
				SetDoer().SetDoerName("username").
				SetRepository().
				SetRepositoryName("username/repositoryname"),
			),
		},
	},
	sequenceFilter: []string{
		"APIAuthorization",
		"RepoAccess",
		"ReqIssueUnlockedOrCanWrite",
	},
	fulfillNeeds: func(t *testing.T, data *testData) {
		t.Helper()
		data.SetDefault("issue", "issueOne")
		data.SetDefault("issueAuthor", "issueAuthor")
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		if data.Has("issue") {
			issueAuthor := fixtureCreateUser(t, &user_model.User{Name: data.Get("issueAuthor")})
			issue := fixtureSetIssue(t, permissions, data.Get("issue"), issueAuthor.Name)
			if data.Has("issueLocked") {
				if data.Get("issueLocked") == "true" {
					fixtureLockIssue(t, permissions, issue)
				} else {
					fixtureUnlockIssue(t, permissions, issue)
				}
			}
		}
		fixtureDisableUnits(t, permissions, data.shared.RepositoryDisabledUnits())
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		t.Helper()
		issue := fixtureGetIssue(t, data.Get("issue"))
		apiv1_permissions.ReqIssueUnlockedOrCanWrite(ctx, issue)
	},
})
