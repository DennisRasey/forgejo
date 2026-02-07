// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package method

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	auth_model "forgejo.org/models/auth"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/log"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/services/auth"
	"forgejo.org/services/auth/source/oauth2"
)

// Ensure the struct implements the interface.
var (
	_ auth.Method = &OAuth2{}
)

// grantAdditionalScopes returns valid scopes coming from grant
func grantAdditionalScopes(grantScopes string) string {
	// scopes_supported from templates/user/auth/oidc_wellknown.tmpl
	scopesSupported := []string{
		"openid",
		"profile",
		"email",
		"groups",
	}

	var apiTokenScopes []string
	for apiTokenScope := range strings.SplitSeq(grantScopes, " ") {
		if slices.Index(scopesSupported, apiTokenScope) == -1 {
			apiTokenScopes = append(apiTokenScopes, apiTokenScope)
		}
	}

	if len(apiTokenScopes) == 0 {
		return ""
	}

	var additionalGrantScopes []string
	allScopes := auth_model.AccessTokenScope("all")

	for _, apiTokenScope := range apiTokenScopes {
		grantScope := auth_model.AccessTokenScope(apiTokenScope)
		if ok, _ := allScopes.HasScope(grantScope); ok {
			additionalGrantScopes = append(additionalGrantScopes, apiTokenScope)
		} else if apiTokenScope == "public-only" {
			additionalGrantScopes = append(additionalGrantScopes, apiTokenScope)
		}
	}
	if len(additionalGrantScopes) > 0 {
		return strings.Join(additionalGrantScopes, ",")
	}

	return ""
}

// OAuth2 implements the Auth interface and authenticates requests (API requests only) by looking for an OAuth token in
// query parameters or the "Authorization" header.
type OAuth2 struct{}

func (o *OAuth2) Verify(req *http.Request, w http.ResponseWriter, _ auth.SessionStore) auth.MethodOutput {
	if !setting.OAuth2.Enabled {
		return &auth.AuthenticationNotAttempted{}
	}

	maybeAuthToken := o.getTokenFromRequest(req)
	if !maybeAuthToken.Has() {
		return &auth.AuthenticationNotAttempted{}
	}
	_, authToken := maybeAuthToken.Get()

	var token oauth2.Token
	_, err := oauth2.DefaultVerifier.ParseWithClaims(authToken, &token)
	if err != nil {
		log.Trace("oauth2.DefaultVerifier.ParseWithClaims: %v", err)
		return &auth.AuthenticationAttemptedIncorrectCredential{Error: err}
	}

	var grant *auth_model.OAuth2Grant
	if grant, err = auth_model.GetOAuth2GrantByID(req.Context(), token.GrantID); err != nil {
		return &auth.AuthenticationError{Error: fmt.Errorf("oauth2 GetOAuth2GrantByID: %w", err)}
	} else if grant == nil {
		return &auth.AuthenticationAttemptedIncorrectCredential{Error: errors.New("oauth2 grant not found or revoked")}
	}
	if token.Type != oauth2.TypeAccessToken {
		return &auth.AuthenticationAttemptedIncorrectCredential{Error: errors.New("token was not an oauth2 access token")}
	}
	if token.ExpiresAt.Before(time.Now()) || token.IssuedAt.After(time.Now()) {
		return &auth.AuthenticationAttemptedIncorrectCredential{Error: errors.New("token was expired")}
	}
	if grant.UserID == 0 {
		return &auth.AuthenticationError{Error: errors.New("oauth2 invalid grant user id")}
	}

	var accessTokenScope optional.Option[auth_model.AccessTokenScope]
	grantScopes := grantAdditionalScopes(grant.Scope)
	if grantScopes != "" {
		accessTokenScope = optional.Some(auth_model.AccessTokenScope(grantScopes))
	} else {
		accessTokenScope = optional.Some(auth_model.AccessTokenScopeAll) // fallback to all
	}

	user, err := user_model.GetPossibleUserByID(req.Context(), grant.UserID)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			return &auth.AuthenticationError{Error: fmt.Errorf("oauth2 GetPossibleUserByID: %w", err)}
		}
		return &auth.AuthenticationAttemptedIncorrectCredential{Error: errors.New("oauth2 grant owner does not exist")}
	}

	return &auth.AuthenticationSuccess{
		Result: &oAuth2JWTAuthenticationResult{
			user:        user,
			scope:       accessTokenScope,
			grantScopes: grantScopes,
		},
	}
}

func (*OAuth2) getTokenFromRequest(req *http.Request) optional.Option[string] {
	if has, token := tokenFromForm(req).Get(); has {
		return optional.Some(token)
	}
	if has, token := tokenFromAuthorizationBasic(req).Get(); has {
		return optional.Some(token)
	}
	if has, token := tokenFromAuthorizationBearer(req).Get(); has {
		return optional.Some(token)
	}
	return optional.None[string]()
}
