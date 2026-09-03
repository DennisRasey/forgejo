// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"reflect"
	"slices"
	"sync"

	"forgejo.org/models/moderation"
	"forgejo.org/modules/json"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm/names"
)

// UserData represents a trimmed down user that is used for preserving
// only the fields needed for abusive content reports (mainly string fields).
type UserData struct { //revive:disable-line:exported
	Name        string
	FullName    string
	Email       string
	LoginName   string
	Location    string
	Website     string
	Pronouns    string
	Description string
	CreatedUnix timeutil.TimeStamp
	UpdatedUnix timeutil.TimeStamp
	// This field was intentionally renamed so that is not the same with the one from User struct.
	// If we keep it the same as in User, during login it might trigger the creation of a shadow copy.
	// TODO: Should we decide that this field is not that relevant for abuse reporting purposes, better remove it.
	LastLogin   timeutil.TimeStamp `json:"LastLoginUnix"`
	Avatar      string
	AvatarEmail string
}

// Implements GetFieldsMap() from ShadowCopyData interface, returning a list of <key, value> pairs
// to be used when rendering the shadow copy for admins reviewing the corresponding abuse report(s).
func (ud UserData) GetFieldsMap() []moderation.ShadowCopyField {
	return []moderation.ShadowCopyField{
		{Key: "Name", Value: ud.Name},
		{Key: "FullName", Value: ud.FullName},
		{Key: "Email", Value: ud.Email},
		{Key: "LoginName", Value: ud.LoginName},
		{Key: "Location", Value: ud.Location},
		{Key: "Website", Value: ud.Website},
		{Key: "Pronouns", Value: ud.Pronouns},
		{Key: "Description", Value: ud.Description},
		{Key: "CreatedUnix", Value: ud.CreatedUnix.AsLocalTime().String()},
		{Key: "UpdatedUnix", Value: ud.UpdatedUnix.AsLocalTime().String()},
		{Key: "LastLogin", Value: ud.LastLogin.AsLocalTime().String()},
		{Key: "Avatar", Value: ud.Avatar},
		{Key: "AvatarEmail", Value: ud.AvatarEmail},
	}
}

// Implements GetAbuserID() from ShadowCopyData interface, returning (GhostUserID, false), since for users/organizations
// the ID is not saved within the shadow copy (because it is already stored as ContentID in the abuse report).
func (ud *UserData) GetAbuserID() (int64, bool) {
	return GhostUserID, false
}

// newUserData creates a trimmed down user to be used just to create a JSON structure
// (keeping only the fields relevant for moderation purposes)
func newUserData(user *User) UserData {
	return UserData{
		Name:        user.Name,
		FullName:    user.FullName,
		Email:       user.Email,
		LoginName:   user.LoginName,
		Location:    user.Location,
		Website:     user.Website,
		Pronouns:    user.Pronouns,
		Description: user.Description,
		CreatedUnix: user.CreatedUnix,
		UpdatedUnix: user.UpdatedUnix,
		LastLogin:   user.LastLoginUnix,
		Avatar:      user.Avatar,
		AvatarEmail: user.AvatarEmail,
	}
}

// userDataColumnNames builds (only once) and returns a slice with the column names
// (e.g. FieldName -> field_name) corresponding to UserData struct fields.
var userDataColumnNames = sync.OnceValue(func() []string {
	mapper := new(names.GonicMapper)
	udType := reflect.TypeFor[UserData]()
	columnNames := make([]string, 0, udType.NumField())
	for field := range udType.Fields() {
		columnNames = append(columnNames, mapper.Obj2Table(field.Name))
	}
	return columnNames
})

// IfNeededCreateShadowCopyForUser checks if for the given user there are any reports of abusive content submitted
// and if found a shadow copy of relevant user fields will be stored into DB and linked to the above report(s).
// This function should be called before a user is deleted or updated.
//
// In case the User object was already altered before calling this method, just provide the userID and
// nil for unalteredUser; when it is decided that a shadow copy should be created and unalteredUser is nil,
// the user will be retrieved from DB based on the provided userID.
//
// For deletions alteredCols argument must be omitted.
//
// In case of updates it will first checks whether any of the columns being updated (alteredCols argument)
// is relevant for moderation purposes (i.e. included in the UserData struct).
func IfNeededCreateShadowCopyForUser(ctx context.Context, userID int64, unalteredUser *User, alteredCols ...string) error {
	// TODO: this can be triggered quite often (e.g. by routers/web/repo/middlewares.go SetDiffViewStyle())

	shouldCheckIfNeeded := len(alteredCols) == 0 // no columns being updated, therefore a deletion
	if !shouldCheckIfNeeded {
		// for updates we need to go further only if certain columns are being changed
		for _, colName := range userDataColumnNames() {
			if shouldCheckIfNeeded = slices.Contains(alteredCols, colName); shouldCheckIfNeeded {
				break
			}
		}
	}

	if !shouldCheckIfNeeded {
		return nil
	}

	shadowCopyNeeded, err := moderation.IsShadowCopyNeeded(ctx, moderation.ReportedContentTypeUser, userID)
	if err != nil {
		return err
	}

	if shadowCopyNeeded {
		if unalteredUser == nil {
			if unalteredUser, err = GetUserByID(ctx, userID); err != nil {
				return err
			}
		}
		userData := newUserData(unalteredUser)
		content, err := json.Marshal(userData)
		if err != nil {
			return err
		}
		return moderation.CreateShadowCopyForUser(ctx, userID, string(content))
	}

	return nil
}
