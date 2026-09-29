// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"slices"

	"forgejo.org/models/auth"
	"forgejo.org/models/db"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/container"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/timeutil"

	"xorm.io/builder"
)

// MembershipReason defines why a user is member of a team/org
type MembershipReason int

const (
	// MembershipReasonUnknown represents a membership created before Forgejo started to track membership provenance
	MembershipReasonUnknown MembershipReason = iota // Represented as zero to match the default values created by the SQL migration

	// MembershipReasonOrgCreator represents a membership that was induced by creating the organization
	MembershipReasonOrgCreator // 1

	// MembershipReasonAddedByUser represents a membership created by another user (already member of the organization or site admin)
	MembershipReasonAddedByUser // 2

	// MembershipReasonAddedByAuthProvider represents a membership created by an authentication source whose metadata was mapped to team membership
	MembershipReasonAddedByAuthProvider // 3
)

// TeamUser represents an team-user relation.
type TeamUser struct {
	ID                     int64            `xorm:"pk autoincr"`
	OrgID                  int64            `xorm:"INDEX"`
	TeamID                 int64            `xorm:"UNIQUE(s)"`
	UID                    int64            `xorm:"UNIQUE(s)"`
	User                   *user_model.User `xorm:"-"`
	CreatedUnix            optional.Option[timeutil.TimeStamp]
	Reason                 MembershipReason       `xorm:"NOT NULL DEFAULT 0"`
	CreatedByUserID        optional.Option[int64] `xorm:"index REFERENCES(user, id)"`
	CreatedByUser          *user_model.User       `xorm:"-"`
	CreatedByLoginSourceID optional.Option[int64] `xorm:"INDEX REFERENCES(login_source, id)"`
	CreatedByLoginSource   *auth.Source           `xorm:"-"`
}

// IsTeamMember returns true if given user is a member of team.
func IsTeamMember(ctx context.Context, orgID, teamID, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("uid=?", userID).
		Table("team_user").
		Exist()
}

// GetTeamUsersByTeamID returns team users for a team
func GetTeamUsersByTeamID(ctx context.Context, teamID int64) ([]*TeamUser, error) {
	teamUsers := make([]*TeamUser, 0, 10)
	return teamUsers, db.GetEngine(ctx).
		Where("team_id=?", teamID).
		Find(&teamUsers)
}

// SearchMembersOptions holds the search options
type SearchMembersOptions struct {
	db.ListOptions
	TeamID int64
}

func (opts SearchMembersOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.TeamID > 0 {
		cond = cond.And(builder.Eq{"team_id": opts.TeamID})
	}
	return cond
}

// GetTeamMembers returns all members in given team of organization.
func GetTeamMembers(ctx context.Context, opts *SearchMembersOptions) ([]*user_model.User, error) {
	var members []*user_model.User
	sess := db.GetEngine(ctx)
	if opts.TeamID > 0 {
		sess = sess.In("id",
			builder.Select("uid").
				From("team_user").
				Where(builder.Eq{"team_id": opts.TeamID}),
		)
	}
	if opts.PageSize > 0 && opts.Page > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	if err := sess.OrderBy("full_name, name").Find(&members); err != nil {
		return nil, err
	}
	return members, nil
}

// IsUserInTeams returns if a user in some teams
func IsUserInTeams(ctx context.Context, userID int64, teamIDs []int64) (bool, error) {
	return db.GetEngine(ctx).Where("uid=?", userID).In("team_id", teamIDs).Exist(new(TeamUser))
}

// GetTeamMemberships returns all team memberships including provenance information.
func GetTeamMemberships(ctx context.Context, opts *SearchMembersOptions) ([]*TeamUser, error) {
	var memberships []*TeamUser
	sess := db.GetEngine(ctx).Table("team_user").Join("inner", []string{"user", "u"}, "u.id = team_user.uid").Where(opts.ToConds())
	if !opts.ListAll && opts.PageSize > 0 && opts.Page > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	if err := sess.OrderBy("u.full_name, u.name").Find(&memberships); err != nil {
		return nil, err
	}

	// Load the members and inviting users
	memberIDs := container.FilterSlice(memberships, func(m *TeamUser) (int64, bool) {
		return m.UID, true
	})
	inviterIDs := container.FilterSlice(memberships, func(m *TeamUser) (int64, bool) {
		has, id := m.CreatedByUserID.Get()
		return id, has && user_model.IsValidUserID(id)
	})

	usersMap, err := db.GetByIDs(ctx, "id", slices.Concat(memberIDs, inviterIDs), &user_model.User{})
	if err != nil {
		return nil, err
	}

	// Load the login sources
	loginSourceIDs := container.FilterSlice(memberships, func(m *TeamUser) (int64, bool) {
		has, id := m.CreatedByLoginSourceID.Get()
		return id, has
	})
	loginSourcesMap, err := db.GetByIDs(ctx, "id", loginSourceIDs, &auth.Source{})
	if err != nil {
		return nil, err
	}

	// Add the users and login sources to the TeamUser objects
	for _, membership := range memberships {
		_, user := user_model.GetUserFromMap(membership.UID, usersMap)
		membership.User = user
		if _, id := membership.CreatedByUserID.Get(); membership.IsMembershipAddedByUser() {
			_, user := user_model.GetUserFromMap(id, usersMap)
			membership.CreatedByUser = user
		}
		if has, id := membership.CreatedByLoginSourceID.Get(); has {
			membership.CreatedByLoginSource = loginSourcesMap[id]
		}
	}

	return memberships, nil
}

func (t *TeamUser) IsUnknownMembershipReason() bool {
	return t.Reason == MembershipReasonUnknown
}

func (t *TeamUser) IsMembershipAsOrgFounder() bool {
	return t.Reason == MembershipReasonOrgCreator
}

func (t *TeamUser) IsMembershipAddedByUser() bool {
	return t.Reason == MembershipReasonAddedByUser
}

func (t *TeamUser) IsMembershipAddedByLoginSource() bool {
	return t.Reason == MembershipReasonAddedByAuthProvider
}
