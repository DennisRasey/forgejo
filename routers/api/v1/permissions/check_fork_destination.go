// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package permissions

import (
	"fmt"
	"net/http"

	"forgejo.org/models/organization"
)

func CheckForkDestination(ctx Context, organizationName *string) {
	if organizationName == nil {
		return
	}
	org, err := organization.GetOrgByName(ctx.GetContext(), *organizationName)
	if err != nil {
		if organization.IsErrOrgNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetOrgByName", err)
		}
		return
	}
	isMember, err := org.IsOrgMember(ctx.GetContext(), ctx.GetDoer().ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsOrgMember", err)
		return
	} else if !isMember {
		ctx.Error(http.StatusForbidden, "isMemberNot", fmt.Sprintf("User is no Member of Organisation '%s'", org.Name))
		return
	}
	if !IsUserSiteAdmin(ctx) {
		canCreate, err := org.CanCreateOrgRepo(ctx.GetContext(), ctx.GetDoer().ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "CanCreateOrgRepo", err)
			return
		}
		if !canCreate {
			ctx.Error(http.StatusForbidden, "CanCreateOrgRepo", fmt.Sprintf("User is not allowed to create repos in Organisation '%s'", org.Name))
			return
		}
	}
}
