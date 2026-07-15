// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package permissions

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	auth_model "forgejo.org/models/auth"
	org_model "forgejo.org/models/organization"
	"forgejo.org/models/perm"
	access_model "forgejo.org/models/perm/access"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	api "forgejo.org/modules/structs"
	"forgejo.org/services/auth"
	"forgejo.org/services/authz"
)

type Permissions struct {
	ctx context.Context

	token                   *auth_model.AccessToken
	doer                    *user_model.User
	user                    *user_model.User
	team                    *org_model.Team
	org                     *org_model.Organization
	isSigned                bool
	authentication          auth.AuthenticationResult
	requiredScopeCategories []auth_model.AccessTokenScopeCategory
	reducer                 authz.AuthorizationReducer
	publicOnly              bool
	repository              *repo_model.Repository
	permission              *access_model.Permission
	packageOwner            *user_model.User
	packageAccessMode       perm.AccessMode

	status  int
	message string
}

func (o *Permissions) Context() context.Context {
	return o.ctx
}

func (o *Permissions) SetContext(ctx context.Context) {
	o.ctx = ctx
}

func (o *Permissions) GetToken() *auth_model.AccessToken {
	return o.token
}

func (o *Permissions) SetToken(token *auth_model.AccessToken) {
	o.token = token
}

func (o *Permissions) Repository() *repo_model.Repository {
	return o.repository
}

func (o *Permissions) SetRepository(repository *repo_model.Repository) {
	o.repository = repository
}

func (o *Permissions) Doer() *user_model.User {
	return o.doer
}

func (o *Permissions) SetDoer(doer *user_model.User) {
	o.doer = doer
}

func (o *Permissions) User() *user_model.User {
	return o.user
}

func (o *Permissions) SetUser(user *user_model.User) {
	o.user = user
}

func (o *Permissions) Organization() *org_model.Organization {
	return o.org
}

func (o *Permissions) SetOrganization(org *org_model.Organization) {
	o.org = org
}

func (o *Permissions) Team() *org_model.Team {
	return o.team
}

func (o *Permissions) SetTeam(team *org_model.Team) {
	o.team = team
}

func (o *Permissions) PackageOwner() *user_model.User {
	return o.packageOwner
}

func (o *Permissions) SetPackageOwner(packageOwner *user_model.User) {
	o.packageOwner = packageOwner
}

func (o *Permissions) PackageAccessMode() perm.AccessMode {
	return o.packageAccessMode
}

func (o *Permissions) SetPackageAccessMode(packageAccessMode perm.AccessMode) {
	o.packageAccessMode = packageAccessMode
}

func (o *Permissions) Permission() *access_model.Permission {
	return o.permission
}

func (o *Permissions) SetPermission(permission *access_model.Permission) {
	o.permission = permission
}

func (o *Permissions) IsSigned() bool {
	return o.isSigned
}

func (o *Permissions) SetIsSigned(isSigned bool) {
	o.isSigned = isSigned
}

func (o *Permissions) PublicOnly() bool {
	return o.publicOnly
}

func (o *Permissions) SetPublicOnly(publicOnly bool) {
	o.publicOnly = publicOnly
}

func (o *Permissions) Reducer() authz.AuthorizationReducer {
	return o.reducer
}

func (o *Permissions) SetReducer(reducer authz.AuthorizationReducer) {
	o.reducer = reducer
}

func (o *Permissions) Authentication() auth.AuthenticationResult {
	return o.authentication
}

func (o *Permissions) SetAuthentication(authentication auth.AuthenticationResult) {
	o.authentication = authentication
}

func (o *Permissions) RequiredScopeCategories() []auth_model.AccessTokenScopeCategory {
	return o.requiredScopeCategories
}

func (o *Permissions) SetRequiredScopeCategories(requiredScopeCategories []auth_model.AccessTokenScopeCategory) {
	o.requiredScopeCategories = requiredScopeCategories
}

func (o *Permissions) GetStatus() int {
	return o.status
}

func (o *Permissions) SetStatus(status int) {
	o.status = status
}

func (o *Permissions) GetMessage() string {
	return o.message
}

func (o *Permissions) SetMessage(message string) {
	o.message = message
}

func (o *Permissions) GetError() error {
	if o.status == 0 {
		return nil
	}
	return fmt.Errorf("%d: %s", o.status, o.message)
}

func (o *Permissions) Error(status int, title string, obj any) {
	var message string
	if err, ok := obj.(error); ok {
		message = err.Error()
	} else {
		message = fmt.Sprintf("%s", obj)
	}
	o.status = status
	o.message = fmt.Sprintf("%s: %s", title, message)
}

func (o *Permissions) NotFound(objs ...any) {
	errors := make([]string, 0)
	message := "Not Found"
	for _, obj := range objs {
		// Ignore nil
		if obj == nil {
			continue
		}

		if err, ok := obj.(error); ok {
			errors = append(errors, err.Error())
		} else {
			message = obj.(string)
		}
	}
	o.Error(http.StatusNotFound, message, strings.Join(errors, ","))
}

func (o *Permissions) InternalServerError(err error) {
	o.Error(http.StatusInternalServerError, "InternalServerError", err.Error())
}

func (o *Permissions) WrittenStatus() int {
	return o.status
}

func (o *Permissions) String() string {
	return strings.Join(o.Strings(), " ")
}

func (o *Permissions) Strings() []string {
	var s []string
	if o.token != nil {
		s = append(s, fmt.Sprintf("%T(ID=%d Token=%s)", o.token, o.token.ID, o.token.Token))
	}
	if o.doer != nil {
		s = append(s, fmt.Sprintf("doer=%T(Name=%s, Private=%v, Admin=%v)", o.doer, o.doer.Name, o.doer.Visibility != api.VisibleTypePublic, o.doer.IsAdmin))
	}
	if o.user != nil {
		s = append(s, fmt.Sprintf("user=%T(Name=%s, Private=%v, Admin=%v)", o.user, o.user.Name, o.user.Visibility != api.VisibleTypePublic, o.user.IsAdmin))
	}
	if o.team != nil {
		s = append(s, fmt.Sprintf("%T(Name=%s)", o.team, o.team.Name))
	}
	if o.org != nil {
		s = append(s, fmt.Sprintf("org=%T(Name=%s)", o.org, o.org.Name))
	}
	if o.isSigned {
		s = append(s, fmt.Sprintf("isSigned(%v)", o.isSigned))
	}
	if o.authentication != nil {
		var sa []string
		user := o.authentication.User()
		if user != nil {
			sa = append(sa, fmt.Sprintf("%T(Name=%s)", user, user.Name))
		}
		if has, scope := o.authentication.Scope().Get(); has {
			sa = append(sa, fmt.Sprintf("%T(%s)", scope, scope))
		}
		if o.authentication.Reducer() != nil {
			sa = append(sa, fmt.Sprintf("%T", o.authentication.Reducer()))
		}
		if o.authentication.IsPasswordAuthentication() {
			sa = append(sa, fmt.Sprintf("IsPasswordAuthentication(%v)", o.authentication.IsPasswordAuthentication()))
		}
		if o.authentication.IsReverseProxyAuthentication() {
			sa = append(sa, fmt.Sprintf("IsReverseProxyAuthentication(%v)", o.authentication.IsReverseProxyAuthentication()))
		}
		if has, oauth2scopes := o.authentication.OAuth2GrantScopes().Get(); has {
			sa = append(sa, fmt.Sprintf("%T(%s)", oauth2scopes, oauth2scopes))
		}
		if has, taskID := o.authentication.ActionsTaskID().Get(); has {
			sa = append(sa, fmt.Sprintf("ActionsTaskID(%d)", taskID))
		}
		s = append(s, fmt.Sprintf("%T(%s)", o.authentication, strings.Join(sa, " ")))
	}
	if len(o.requiredScopeCategories) > 0 {
		s = append(s, fmt.Sprintf("%T(%v)", o.requiredScopeCategories, o.requiredScopeCategories))
	}
	if o.reducer != nil {
		s = append(s, fmt.Sprintf("%T", o.reducer))
	}
	if o.publicOnly {
		s = append(s, fmt.Sprintf("publicOnly(%v)", o.publicOnly))
	}
	if o.repository != nil {
		var sa []string
		if o.repository.IsPrivate {
			sa = append(sa, fmt.Sprintf("IsPrivate(%v)", o.repository.IsPrivate))
		}
		if o.repository.IsArchived {
			sa = append(sa, fmt.Sprintf("IsArchived(%v)", o.repository.IsArchived))
		}
		if o.repository.IsMirror {
			sa = append(sa, fmt.Sprintf("IsMirror(%v)", o.repository.IsMirror))
		}
		if len(o.repository.Units) > 0 {
			var units []string
			for _, repoUnit := range o.repository.Units {
				units = append(units, repoUnit.Type.String())
			}
			sa = append(sa, fmt.Sprintf("Unit(%s)", strings.Join(units, ",")))
		}
		s = append(s, fmt.Sprintf("%T(ID=%d Name=%s OwnerName=%s %s)", o.repository, o.repository.ID, o.repository.Name, o.repository.OwnerName, strings.Join(sa, " ")))
	}
	if o.permission != nil {
		var sa []string
		if len(o.permission.Units) > 0 {
			var units []string
			for _, repoUnit := range o.permission.Units {
				unitString := repoUnit.Type.String()
				if o.permission.UnitsMode != nil {
					if mode, has := o.permission.UnitsMode[repoUnit.Type]; has {
						unitString = fmt.Sprintf("%s:%s", unitString, mode)
					}
				}
				units = append(units, unitString)
			}
			sa = append(sa, fmt.Sprintf("Unit(%s)", strings.Join(units, ",")))
		}
		sa = append(sa, fmt.Sprintf("AccessMode(%s)", o.permission.AccessMode))
		s = append(s, fmt.Sprintf("%T(%s)", o.permission, strings.Join(sa, " ")))
	}
	if o.packageOwner != nil {
		s = append(s, fmt.Sprintf("packageOwner %T(Name=%s)", o.packageOwner, o.packageOwner.Name))
		s = append(s, fmt.Sprintf("packageMode %T(%s)", o.packageAccessMode, o.packageAccessMode))
	}
	return s
}
