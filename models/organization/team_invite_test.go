// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/models/organization"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamInvite(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Service.TeamInvitationExpiryDays, 14)()

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})

	t.Run("MailExistsInTeam", func(t *testing.T) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// user 2 already added to team 2, should result in error
		_, err := organization.CreateTeamInviteByEmail(db.DefaultContext, user2, team, user2.Email)
		require.Error(t, err)
	})

	t.Run("UserExistsInTeam", func(t *testing.T) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// user 4 already added to team 2, should result in error
		_, err := organization.CreateTeamInviteForUser(db.DefaultContext, user2, user4, team)
		require.Error(t, err)
	})

	t.Run("CreateAndRemoveByUser", func(t *testing.T) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

		invite, err := organization.CreateTeamInviteForUser(db.DefaultContext, user1, user5, team)
		assert.NotNil(t, invite)
		require.NoError(t, err)
		hasExpiration, expirationDate := invite.ExpiryUnix.Get()
		assert.True(t, hasExpiration)
		assert.Greater(t, expirationDate, timeutil.TimeStampNow().AddDuration(13*24*time.Hour))
		assert.Less(t, expirationDate, timeutil.TimeStampNow().AddDuration(15*24*time.Hour))

		// Shouldn't allow duplicate invite by email
		_, err = organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, user5.Email)
		require.Error(t, err)
		// Shouldn't allow duplicate invite by user
		_, err = organization.CreateTeamInviteForUser(db.DefaultContext, user1, user5, team)
		require.Error(t, err)

		// Check that the invite is visible through various getters
		singleInvite, err := organization.GetInviteByOrgAndUser(db.DefaultContext, team.OrgID, user5.ID)
		require.NoError(t, err)
		assert.Equal(t, invite.ID, singleInvite.ID)
		teams, err := organization.GetTeamsInvitedTo(db.DefaultContext, team.OrgID, user5.ID)
		require.NoError(t, err)
		assert.Len(t, teams, 1)
		assert.Equal(t, team, teams[0])

		// should remove invite
		require.NoError(t, organization.RemoveInviteByID(db.DefaultContext, invite.ID, invite.TeamID))

		// invite should not exist
		_, err = organization.GetInviteByToken(db.DefaultContext, invite.Token)
		require.Error(t, err)
	})

	t.Run("CreateByEmailAndRemove", func(t *testing.T) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		invite, err := organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, "org3@example.com")
		assert.NotNil(t, invite)
		require.NoError(t, err)
		hasExpiration, expirationDate := invite.ExpiryUnix.Get()
		assert.True(t, hasExpiration)
		assert.Greater(t, expirationDate, timeutil.TimeStampNow().AddDuration(13*24*time.Hour))
		assert.Less(t, expirationDate, timeutil.TimeStampNow().AddDuration(15*24*time.Hour))

		// Shouldn't allow duplicate invite
		_, err = organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, "org3@example.com")
		require.Error(t, err)

		// should remove invite
		require.NoError(t, organization.RemoveInviteByID(db.DefaultContext, invite.ID, invite.TeamID))

		// invite should not exist
		_, err = organization.GetInviteByToken(db.DefaultContext, invite.Token)
		require.Error(t, err)
	})

	t.Run("CreateByUserWithoutExpiration", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.TeamInvitationExpiryDays, 0)()
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

		invite, err := organization.CreateTeamInviteForUser(db.DefaultContext, user1, user5, team)
		require.NoError(t, err)
		assert.NotNil(t, invite)
		assert.False(t, invite.ExpiryUnix.Has())

		// Shouldn't allow duplicate invite by email
		_, err = organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, user5.Email)
		require.Error(t, err)
		// Shouldn't allow duplicate invite by user
		_, err = organization.CreateTeamInviteForUser(db.DefaultContext, user1, user5, team)
		require.Error(t, err)
	})

	t.Run("CreateByEmailWithoutExpiration", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.TeamInvitationExpiryDays, 0)()
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		invite, err := organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, "org3@example.com")
		assert.NotNil(t, invite)
		require.NoError(t, err)
		assert.False(t, invite.ExpiryUnix.Has())

		// Shouldn't allow duplicate invite
		_, err = organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, "org3@example.com")
		require.Error(t, err)
	})

	t.Run("RecreateByUserAfterExpiration", func(t *testing.T) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user12 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})

		invite, err := organization.CreateTeamInviteForUser(db.DefaultContext, user1, user12, team)
		assert.NotNil(t, invite)
		require.NoError(t, err)
		// manually make the invite expire
		_, err = db.GetEngine(t.Context()).Table("team_invite").Cols("expiry_unix").Update(
			&organization.TeamInvite{ExpiryUnix: optional.Some(timeutil.TimeStamp(int64(timeutil.TimeStampNow()) - 500))},
		)
		require.NoError(t, err)

		// Creating the invite again succeeds
		newInvite, err := organization.CreateTeamInviteForUser(db.DefaultContext, user1, user12, team)
		require.NoError(t, err)
		assert.Equal(t, newInvite.InvitedUser, user12)
		assert.False(t, newInvite.IsExpired())

		// The previous invite is deleted
		oldInviteExists, err := db.GetEngine(t.Context()).Exist(&organization.TeamInvite{ID: invite.ID})
		require.NoError(t, err)
		assert.False(t, oldInviteExists)
	})

	t.Run("RecreateByEmailAfterExpiration", func(t *testing.T) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		invite, err := organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, "hello@example.com")
		assert.NotNil(t, invite)
		require.NoError(t, err)
		// manually make the invite expire
		_, err = db.GetEngine(t.Context()).Table("team_invite").Cols("expiry_unix").Update(
			&organization.TeamInvite{ExpiryUnix: optional.Some(timeutil.TimeStamp(int64(timeutil.TimeStampNow()) - 500))},
		)
		require.NoError(t, err)

		// Creating the invite again succeeds
		newInvite, err := organization.CreateTeamInviteByEmail(db.DefaultContext, user1, team, "hello@example.com")
		require.NoError(t, err)
		assert.Equal(t, "hello@example.com", newInvite.Email)
		assert.False(t, newInvite.IsExpired())

		// The previous invite is deleted
		oldInviteExists, err := db.GetEngine(t.Context()).Exist(&organization.TeamInvite{ID: invite.ID})
		require.NoError(t, err)
		assert.False(t, oldInviteExists)
	})
}
