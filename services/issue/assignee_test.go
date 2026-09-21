// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	"forgejo.org/models/organization"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteNotPassedAssignee(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Fake issue with assignees
	issue, err := issues_model.GetIssueByID(db.DefaultContext, 1)
	require.NoError(t, err)

	err = issue.LoadAttributes(db.DefaultContext)
	require.NoError(t, err)

	assert.Len(t, issue.Assignees, 1)

	user1, err := user_model.GetUserByID(db.DefaultContext, 1) // This user is already assigned (see the definition in fixtures), so running  UpdateAssignee should unassign him
	require.NoError(t, err)

	// Check if he got removed
	isAssigned, err := issues_model.IsUserAssignedToIssue(db.DefaultContext, issue, user1)
	require.NoError(t, err)
	assert.True(t, isAssigned)

	// Clean everyone
	err = DeleteNotPassedAssignee(db.DefaultContext, issue, user1, []*user_model.User{})
	require.NoError(t, err)
	assert.Empty(t, issue.Assignees)

	// Reload to check they're gone
	issue.ResetAttributesLoaded()
	require.NoError(t, issue.LoadAssignees(db.DefaultContext))
	assert.Empty(t, issue.Assignees)
	assert.Empty(t, issue.Assignee)
}

func TestIsValidTeamReviewRequest(t *testing.T) {
	defer unittest.OverrideFixtures("services/issue/TestIsValidTeamReviewRequest")()
	require.NoError(t, unittest.PrepareTestDatabase())

	// issue 900 / repo 900: a private PR on an org-owned repo (org 900),
	// authored by user 902. Team 900 ("review_team_900") has read access to
	// repo 900 and to the pull request unit; user 901's only access to the
	// repo comes through membership in that team. Team 901 has no access to
	// repo 900 at all. User 903 has no relation to the repo or either team.
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 900})

	// Mirrors apiReviewRequest exactly: Issue.LoadRepo populates issue.Repo
	// but deliberately does NOT preload issue.Repo.Owner.
	require.NoError(t, issue.LoadRepo(t.Context()))
	assert.Nil(t, issue.Repo.Owner, "fixture setup should leave Owner unloaded, matching production")

	teamWithRepoAccess := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 900})
	teamWithoutRepoAccess := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 901})

	poster := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 902})
	teamOnlyMember := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 901}) // only access is via teamWithRepoAccess membership
	unrelatedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 903})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 900})

	tests := []struct {
		name       string
		reviewer   *organization.Team
		doer       *user_model.User
		isAdd      bool
		wantReason string // empty means the request is expected to be valid
	}{
		{
			name:     "poster can add a team reviewer that has repo access",
			reviewer: teamWithRepoAccess,
			doer:     poster,
			isAdd:    true,
		},
		{
			name:     "doer whose only access is via team membership can add that team as a reviewer",
			reviewer: teamWithRepoAccess,
			doer:     teamOnlyMember,
			isAdd:    true,
		},
		{
			name:       "doer unrelated to the repo cannot add a reviewer",
			reviewer:   teamWithRepoAccess,
			doer:       unrelatedUser,
			isAdd:      true,
			wantReason: "Doer can't choose reviewer",
		},
		{
			name:     "doer whose only access is via team membership can remove that team as a reviewer",
			reviewer: teamWithRepoAccess,
			doer:     teamOnlyMember,
			isAdd:    false,
		},
		{
			name:       "doer unrelated to the repo cannot remove a reviewer",
			reviewer:   teamWithRepoAccess,
			doer:       unrelatedUser,
			isAdd:      false,
			wantReason: "Doer can't remove reviewer",
		},
		{
			name:       "an organization cannot be the doer",
			reviewer:   teamWithRepoAccess,
			doer:       org,
			isAdd:      true,
			wantReason: "Organization can't be doer to add reviewer",
		},
		{
			name:       "a team without repo access cannot be added as a reviewer, even by the poster",
			reviewer:   teamWithoutRepoAccess,
			doer:       poster,
			isAdd:      true,
			wantReason: "Reviewing team can't read repo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsValidTeamReviewRequest(t.Context(), tt.reviewer, tt.doer, tt.isAdd, issue)
			if tt.wantReason == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			var reviewReqErr issues_model.ErrNotValidReviewRequest
			require.ErrorAs(t, err, &reviewReqErr)
			assert.Equal(t, tt.wantReason, reviewReqErr.Reason)
		})
	}
}
