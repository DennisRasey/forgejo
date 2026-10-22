// Copyright 2025 The Forgejo Contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package source

import (
	"context"

	"forgejo.org/models/quota"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/container"
	"forgejo.org/modules/log"
)

func SyncGroupsToQuotaGroups(ctx context.Context, user *user_model.User, sourceUserGroups container.Set[string], sourceGroupQuotaGroupMapping map[string]container.Set[string], performRemoval bool) error {
	qgroupCache := make(map[string]*quota.Group)
	qgroupsToAdd, qgroupsToRemove := resolveMappedQuotaGroups(sourceUserGroups, sourceGroupQuotaGroupMapping)
	if performRemoval {
		if err := syncGroupsToQuotaGroupsCached(ctx, user, qgroupsToRemove, syncRemove, qgroupCache); err != nil {
			return err
		}
	}
	return syncGroupsToQuotaGroupsCached(ctx, user, qgroupsToAdd, syncAdd, qgroupCache)
}

func resolveMappedQuotaGroups(sourceUserGroups container.Set[string], sourceGroupQuotaGroupMapping map[string]container.Set[string]) (container.Set[string], container.Set[string]) {
	qgroupsToAdd := make(container.Set[string])
	qgroupsToRemove := make(container.Set[string])
	for group, qgroups := range sourceGroupQuotaGroupMapping {
		isUserInGroup := sourceUserGroups.Contains(group)
		if isUserInGroup {
			for qgroup := range qgroups {
				qgroupsToAdd[qgroup] = struct{}{}
			}
		} else {
			for qgroup := range qgroups {
				qgroupsToRemove[qgroup] = struct{}{}
			}
		}
	}
	return qgroupsToAdd, qgroupsToRemove
}

func syncGroupsToQuotaGroupsCached(ctx context.Context, user *user_model.User, qgroups container.Set[string], action syncType, qgroupCache map[string]*quota.Group) error {
	for qgroupName := range qgroups {
		var err error
		qgroup, ok := qgroupCache[qgroupName]
		if !ok {
			qgroup, err = quota.GetGroupByName(ctx, qgroupName)
			if err != nil {
				if quota.IsErrGroupNotFound(err) {
					log.Warn("quota group sync: Could not find quota group %s: %v", qgroupName, err)
					continue
				}

				return err
			}

			qgroupCache[qgroup.Name] = qgroup
		}
		isMember, err := qgroup.IsUserInGroup(ctx, user.ID)
		if err != nil {
			return err
		}

		if action == syncAdd && !isMember {
			if err := qgroup.AddUserByID(ctx, user.ID); err != nil {
				log.Error("quota group sync: Could not add user to quota group: %v", err)
				return err
			}
		} else if action == syncRemove && isMember {
			if err := qgroup.RemoveUserByID(ctx, user.ID); err != nil {
				log.Error("quota group sync: Could not remove user from quota group: %v", err)
				return err
			}
		}
	}
	return nil
}
