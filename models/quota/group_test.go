// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package quota_test

import (
	"testing"

	quota_model "forgejo.org/models/quota"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupUpdate(t *testing.T) {
	defer test.MockVariableValue(&setting.Quota.DefaultGroups, []string{"default"})()
	defer unittest.OverrideFixtures("models/quota/fixtures")()
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	defaultGroups, groupList, err := quota_model.GetGroupsForUserMember(ctx, user.ID)
	require.NoError(t, err)

	// the user should be in the default group and no others
	require.Len(t, defaultGroups, 1)
	assert.Equal(t, "default", defaultGroups[0].Group.Name)
	assert.True(t, defaultGroups[0].IsMember)

	for _, g := range groupList {
		assert.False(t, g.IsMember)
	}

	// set a group
	oldQuotaGroups := []string{}
	newQuotaGroups := []string{"trusted-user"}
	err = quota_model.UpdateGroupsForUser(ctx, user.ID, oldQuotaGroups, newQuotaGroups)
	require.NoError(t, err)

	defaultGroups, groupList, err = quota_model.GetGroupsForUserMember(ctx, user.ID)
	require.NoError(t, err)

	// the user should now be in the new group and the default group should no
	// longer apply
	require.Len(t, defaultGroups, 1)
	assert.False(t, defaultGroups[0].IsMember)
	for _, g := range groupList {
		if g.Group.Name == "trusted-user" {
			assert.True(t, g.IsMember)
		} else {
			assert.False(t, g.IsMember)
		}
	}

	// add another group
	oldQuotaGroups = newQuotaGroups
	newQuotaGroups = []string{"trusted-user", "medium"}
	err = quota_model.UpdateGroupsForUser(ctx, user.ID, oldQuotaGroups, newQuotaGroups)
	require.NoError(t, err)

	defaultGroups, groupList, err = quota_model.GetGroupsForUserMember(ctx, user.ID)
	require.NoError(t, err)

	require.Len(t, defaultGroups, 1)
	assert.False(t, defaultGroups[0].IsMember)
	for _, g := range groupList {
		if g.Group.Name == "trusted-user" || g.Group.Name == "medium" {
			assert.True(t, g.IsMember)
		} else {
			assert.False(t, g.IsMember)
		}
	}
}
