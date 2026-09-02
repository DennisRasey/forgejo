// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	user_model "forgejo.org/models/user"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTestWithCall(apiv1_permissions.ReqEditIssue, functionTest{
	testCases: []*testCase{
		{
			// pass because the issue is not locked and
			// title can be modified by the author if the issue
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueFields": "title",
			}, newSharedData().
				SetDoer().SetDoerName("issueAuthor").
				SetRepository(),
			),
		},
		{
			// pass because the issue is not locked and
			// title,body,ref,state can be modified by the author
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueFields": "title,body,ref,state",
			}, newSharedData().
				SetDoer().SetDoerName("issueAuthor").
				SetRepository(),
			),
		},
		{
			// fail because the issue is locked
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueFields": "title,body,ref,state",
				"issueLocked": "true",
			}, newSharedData().
				SetDoer().SetDoerName("issueAuthor").
				SetRepository(),
			),
			error: "No permission to edit the following issue fields: title(locked),body(locked),ref(locked),state(locked)",
		},
		{
			// fail because the author of the issue has no permission
			// to edit those fields
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueFields": "updated_at,assignees,milestone,due_date,unset_due_date",
			}, newSharedData().
				SetDoer().SetDoerName("issueAuthor").
				SetRepository(),
			),
			error: "No permission to edit the following issue fields: updated_at,assignees,milestone,due_date,unset_due_date",
		},
		{
			// pass because a collaborator with write permission on the repository
			// can modify all fields (except the updated_at field)
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueFields": "assignees,milestone,due_date,unset_due_date,title,body,ref,state",
			}, newSharedData().
				SetDoer().SetDoerName("collaborator").
				SetRepository().
				SetRepositoryCollaborator("collaborator"),
			),
		},
		{
			// fail because a collaborator with write permission on the repository
			// cannot modify the updated_at field
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueFields": "updated_at",
			}, newSharedData().
				SetDoer().SetDoerName("collaborator").
				SetRepository().
				SetRepositoryCollaborator("collaborator"),
			),
			error: "No permission to edit the following issue fields: updated_at",
		},
		{
			// pass because the owner of the repository can modify the updated_at field
			data: newTestData(map[string]string{
				"issue":       "issueOne",
				"issueAuthor": "issueAuthor",
				"issueFields": "updated_at",
			}, newSharedData().
				SetDoer().SetDoerName("userowner").
				SetRepository().SetRepositoryName("userowner/repositorypublic"),
			),
		},
	},
	sequenceFilter: []string{
		"APIAuthorization",
		"RepoAccess",
		"ReqEditIssue",
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		issueAuthor := fixtureCreateUser(t, &user_model.User{Name: data.Get("issueAuthor")})

		issue := fixtureSetIssue(t, permissions, data.Get("issue"), issueAuthor.Name)
		if data.Has("issueLocked") {
			if data.Get("issueLocked") == "true" {
				fixtureLockIssue(t, permissions, issue)
			}
		}
	},
	call: func(t *testing.T, ctx apiv1_permissions.Context, data *testData, _ []any) {
		t.Helper()
		issue := fixtureGetIssue(t, data.Get("issue"))
		opt := fixtureEditIssueOption(t, data.Get("issueFields"))
		apiv1_permissions.ReqEditIssue(ctx, issue, &opt)
	},
})
