// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"fmt"
	"time"

	"forgejo.org/models/db"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"

	"xorm.io/builder"
)

type ErrTeamInviteAlreadyExist struct {
	TeamID        int64
	Email         string
	InvitedUserID int64
}

func IsErrTeamInviteAlreadyExist(err error) bool {
	_, ok := err.(ErrTeamInviteAlreadyExist)
	return ok
}

func (err ErrTeamInviteAlreadyExist) Error() string {
	return fmt.Sprintf("team invite already exists [team_id: %d, email: %s, invited_user_id: %d]", err.TeamID, err.Email, err.InvitedUserID)
}

func (err ErrTeamInviteAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

type ErrTeamInviteNotFound struct {
	Token string
}

func IsErrTeamInviteNotFound(err error) bool {
	_, ok := err.(ErrTeamInviteNotFound)
	return ok
}

func (err ErrTeamInviteNotFound) Error() string {
	return fmt.Sprintf("team invite was not found [token: %s]", err.Token)
}

func (err ErrTeamInviteNotFound) Unwrap() error {
	return util.ErrNotExist
}

type ErrTeamInviteExpired struct {
	Token string
}

func IsErrTeamInviteExpired(err error) bool {
	_, ok := err.(ErrTeamInviteExpired)
	return ok
}

func (err ErrTeamInviteExpired) Error() string {
	return fmt.Sprintf("team invite has expired [token: %s]", err.Token)
}

func (err ErrTeamInviteExpired) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrInvitedUserAlreadyAdded indicates that a user is already part of a team and can not be invited again.
type ErrInvitedUserAlreadyAdded struct {
	Email         string
	InvitedUserID optional.Option[int64]
}

// IsErrUserEmailAlreadyAdded checks if an error is a ErrUserEmailAlreadyAdded.
func IsErrUserEmailAlreadyAdded(err error) bool {
	_, ok := err.(ErrInvitedUserAlreadyAdded)
	return ok
}

func (err ErrInvitedUserAlreadyAdded) Error() string {
	return fmt.Sprintf("user with email already added [email: %s, invited_user_id: %d]", err.Email, err.InvitedUserID)
}

func (err ErrInvitedUserAlreadyAdded) Unwrap() error {
	return util.ErrAlreadyExist
}

// TeamInvite represents an invite to a team
type TeamInvite struct {
	ID          int64                               `xorm:"pk autoincr"`
	Token       string                              `xorm:"UNIQUE(token) INDEX NOT NULL DEFAULT ''"`
	InviterID   int64                               `xorm:"NOT NULL DEFAULT 0"`
	OrgID       int64                               `xorm:"INDEX NOT NULL DEFAULT 0"`
	TeamID      int64                               `xorm:"UNIQUE(team_mail) INDEX NOT NULL DEFAULT 0"`
	Email       string                              `xorm:"UNIQUE(team_mail) NOT NULL DEFAULT ''"`
	InvitedID   optional.Option[int64]              `xorm:"index REFERENCES(user, id)"`
	InvitedUser *user_model.User                    `xorm:"-"`
	CreatedUnix timeutil.TimeStamp                  `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp                  `xorm:"INDEX updated"`
	ExpiryUnix  optional.Option[timeutil.TimeStamp] `xorm:"expiry_unix"`
}

// CreateTeamInviteByEmail creates a TeamInvite for someone who does not have an account yet.
func CreateTeamInviteByEmail(ctx context.Context, doer *user_model.User, team *Team, email string) (*TeamInvite, error) {
	existingInvite := TeamInvite{
		TeamID: team.ID,
		Email:  email,
	}
	has, err := db.GetEngine(ctx).Get(&existingInvite)
	if err != nil {
		return nil, err
	}
	if has {
		if existingInvite.IsExpired() {
			_, err := db.GetEngine(ctx).Delete(&existingInvite)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, ErrTeamInviteAlreadyExist{
				TeamID: team.ID,
				Email:  email,
			}
		}
	}

	// check if the user is already a team member by email
	exist, err := db.GetEngine(ctx).
		Where(builder.Eq{
			"team_user.org_id":  team.OrgID,
			"team_user.team_id": team.ID,
			"`user`.email":      email,
		}).
		Join("INNER", "`user`", "`user`.id = team_user.uid").
		Table("team_user").
		Exist()
	if err != nil {
		return nil, err
	}

	if exist {
		return nil, ErrInvitedUserAlreadyAdded{
			Email: email,
		}
	}

	token := util.CryptoRandomString(util.RandomStringMedium)

	invite := &TeamInvite{
		Token:      token,
		InviterID:  doer.ID,
		OrgID:      team.OrgID,
		TeamID:     team.ID,
		Email:      email,
		ExpiryUnix: getInviteExpiry(),
	}

	return invite, db.Insert(ctx, invite)
}

// CreateTeamInviteForUser creates a TeamInvite for someone who already has an account on the instance.
func CreateTeamInviteForUser(ctx context.Context, doer, invited *user_model.User, team *Team) (*TeamInvite, error) {
	existingInvite := TeamInvite{
		TeamID:    team.ID,
		InvitedID: optional.Some(invited.ID),
	}
	has, err := db.GetEngine(ctx).Get(&existingInvite)
	if err != nil {
		return nil, err
	}
	if has {
		if existingInvite.IsExpired() {
			_, err := db.GetEngine(ctx).Delete(&existingInvite)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, ErrTeamInviteAlreadyExist{
				TeamID: team.ID,
				Email:  invited.Email,
			}
		}
	}

	// check if the user is already a team member
	exist, err := db.GetEngine(ctx).
		Where(builder.Eq{
			"org_id":  team.OrgID,
			"team_id": team.ID,
			"uid":     invited.ID,
		}).
		Table("team_user").
		Exist()
	if err != nil {
		return nil, err
	}

	if exist {
		return nil, ErrInvitedUserAlreadyAdded{
			InvitedUserID: optional.Some(invited.ID),
		}
	}

	token := util.CryptoRandomString(util.RandomStringMedium)

	invite := &TeamInvite{
		Token:       token,
		InviterID:   doer.ID,
		OrgID:       team.OrgID,
		TeamID:      team.ID,
		Email:       invited.Email,
		InvitedID:   optional.Some(invited.ID),
		InvitedUser: invited,
		ExpiryUnix:  getInviteExpiry(),
	}

	return invite, db.Insert(ctx, invite)
}

func RemoveInviteByID(ctx context.Context, inviteID, teamID int64) error {
	_, err := db.DeleteByBean(ctx, &TeamInvite{
		ID:     inviteID,
		TeamID: teamID,
	})
	return err
}

func GetInvitesByTeamID(ctx context.Context, teamID int64) ([]*TeamInvite, error) {
	invites := make([]*TeamInvite, 0, 10)
	return invites, db.GetEngine(ctx).
		Where("team_id=?", teamID).
		Find(&invites)
}

func GetInviteByToken(ctx context.Context, token string) (*TeamInvite, error) {
	invite := &TeamInvite{}

	has, err := db.GetEngine(ctx).Where("token=?", token).Get(invite)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrTeamInviteNotFound{Token: token}
	}
	return invite, nil
}

// GetInviteByOrgAndUser finds any non-expired invite for this user to teams of the given org
func GetInviteByOrgAndUser(ctx context.Context, orgID, userID int64) (*TeamInvite, error) {
	invite := &TeamInvite{
		OrgID:     orgID,
		InvitedID: optional.Some(userID),
	}

	has, err := db.GetEngine(ctx).Where("expiry_unix > ? OR expiry_unix = 0", timeutil.TimeStampNow()).Get(invite)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrTeamInviteNotFound{}
	}
	return invite, nil
}

// GetTeamsInvitedTo lists the teams of an organization a user is invited to
func GetTeamsInvitedTo(ctx context.Context, orgID, userID int64) ([]*Team, error) {
	teams := make([]*Team, 0, 10)
	return teams, db.GetEngine(ctx).
		Where(builder.And(
			builder.Eq{
				"team_invite.invited_id": userID,
				"team_invite.org_id":     orgID,
			},
			builder.Expr("team_invite.expiry_unix > ? OR expiry_unix = 0", timeutil.TimeStampNow()),
		)).
		Join("INNER", "`team_invite`", "`team_invite`.team_id = team.id").
		Table("team").
		Find(&teams)
}

func (i *TeamInvite) LoadInvitedUser(ctx context.Context) error {
	if i.InvitedUser == nil {
		hasInvitedUser, userID := i.InvitedID.Get()
		if hasInvitedUser {
			user, err := user_model.GetUserByID(ctx, userID)
			if err != nil {
				return err
			}
			i.InvitedUser = user
		}
	}
	return nil
}

// IsExpired determines if an invite is no longer valid because it expired
func (i *TeamInvite) IsExpired() bool {
	hasExpiry, deadline := i.ExpiryUnix.Get()
	now := timeutil.TimeStampNow()
	return hasExpiry && deadline < now
}

// getInviteExpiry computes the expiration date of an invite created now
func getInviteExpiry() optional.Option[timeutil.TimeStamp] {
	if setting.Service.TeamInvitationExpiryDays == 0 {
		return optional.None[timeutil.TimeStamp]()
	}
	deadline := timeutil.TimeStampNow().AddDuration(time.Duration(setting.Service.TeamInvitationExpiryDays) * 24 * time.Hour)
	return optional.Some(deadline)
}
