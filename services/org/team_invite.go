// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"context"

	"forgejo.org/models"
	org_model "forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/services/mailer"
)

// CreateTeamInviteByEmail makes a persistent invite in db for someone without an account and mails it to them.
func CreateTeamInviteByEmail(ctx context.Context, inviter *user_model.User, team *org_model.Team, uname string) error {
	invite, err := org_model.CreateTeamInviteByEmail(ctx, inviter, team, uname)
	if err != nil {
		return err
	}

	return mailer.MailTeamInvite(ctx, inviter, team, invite)
}

// CreateTeamInviteByUser makes a persistent invite in db for someone with an account already and mails it.
func CreateTeamInviteByUser(ctx context.Context, inviter, invited *user_model.User, team *org_model.Team) error {
	invite, err := org_model.CreateTeamInviteForUser(ctx, inviter, invited, team)
	if err != nil {
		return err
	}
	// TODO: instead of only sending an email, also create an in-app notification
	return mailer.MailTeamInvite(ctx, inviter, team, invite)
}

// InviteOrAddTeamMember invites the user to the team if all team changes should go through invites, or adds them directly otherwise.
func InviteOrAddTeamMember(ctx context.Context, inviter, invited *user_model.User, team *org_model.Team) error {
	if setting.Service.AddMembersByInvitations && inviter.ID != invited.ID {
		return CreateTeamInviteByUser(ctx, inviter, invited, team)
	}
	return models.AddTeamMember(ctx, team, invited.ID)
}

// DeclineInvite turns down an invitation to a team
func DeclineInvite(ctx context.Context, invite *org_model.TeamInvite) error {
	// TODO: notify the inviter here
	return org_model.RemoveInviteByID(ctx, invite.ID, invite.TeamID)
}
