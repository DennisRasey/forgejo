// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"forgejo.org/models/db"
	"forgejo.org/models/forgefed"
	"forgejo.org/models/unittest"
	"forgejo.org/models/user"
	"forgejo.org/modules/activitypub"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/routers"
	"forgejo.org/services/contexttest"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFederationHttpSigValidation(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		userID := 2
		userURL := fmt.Sprintf("%sapi/v1/activitypub/user-id/%d", u, userID)

		user1 := unittest.AssertExistsAndLoadBean(t, &user.User{ID: 1})

		ctx, _ := contexttest.MockAPIContext(t, userURL)
		clientFactory, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		apClient, err := clientFactory.WithKeys(ctx, user1, user1.KeyID(), nil)
		require.NoError(t, err)

		// HACK HACK HACK: the host part of the URL gets set to which IP forgejo is
		// listening on, NOT localhost, which is the Domain given to forgejo which
		// is then used for eg. the keyID all requests
		applicationKeyID := fmt.Sprintf("%sapi/v1/activitypub/actor#main-key", setting.AppURL)
		actorKeyID := fmt.Sprintf("%sapi/v1/activitypub/user-id/1#main-key", setting.AppURL)

		// Unsigned request
		t.Run("UnsignedRequest", func(t *testing.T) {
			req := NewRequest(t, "GET", userURL)
			MakeRequest(t, req, http.StatusBadRequest)
		})

		// Check for missing public keys
		t.Run("ValidateEmptyCaches", func(t *testing.T) {
			_, err := forgefed.FindFederationHostByKeyID(db.DefaultContext, applicationKeyID)
			require.Error(t, err)
			assert.True(t, forgefed.IsErrFederationHostNotFound(err))

			_, _, err = user.FindFederatedUserByKeyID(db.DefaultContext, actorKeyID)
			require.Error(t, err)
			assert.True(t, user.IsErrFederatedUserNotExists(err))
		})

		// Signed request
		t.Run("SignedRequest", func(t *testing.T) {
			resp, err := apClient.Get(userURL)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})

		// Check for cached public keys
		t.Run("ValidateCaches", func(t *testing.T) {
			host, err := forgefed.FindFederationHostByKeyID(db.DefaultContext, applicationKeyID)
			require.NoError(t, err)
			assert.NotNil(t, host)
			assert.True(t, host.PublicKey.Valid)

			_, user, err := user.FindFederatedUserByKeyID(db.DefaultContext, actorKeyID)
			require.NoError(t, err)
			assert.NotNil(t, user)
			assert.True(t, user.PublicKey.Valid)
		})

		// Disable signature validation
		defer test.MockVariableValue(&setting.Federation.SignatureEnforced, false)()

		// Unsigned request
		t.Run("SignatureValidationDisabled", func(t *testing.T) {
			req := NewRequest(t, "GET", userURL)
			MakeRequest(t, req, http.StatusOK)
		})
	})
}

func TestFederationAllRoutesCovered(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	routes := routers.NormalRoutes().R.Routes()

	var r *chi.Route
	for _, route := range routes {
		if route.Pattern == "/api/v1/*" {
			r = &route
			break
		}
	}

	require.NotNil(t, r)

	ranOne := false
	for _, route := range r.SubRoutes.Routes() {
		if !strings.HasPrefix(route.Pattern, "/activitypub/") {
			continue
		}

		ranOne = true
		if route.Pattern == "/activitypub/actor" {
			// unsigned request to the actor should always succed
			req := NewRequest(t, "GET", fmt.Sprintf("%sapi/v1/activitypub/actor", setting.AppURL))
			MakeRequest(t, req, http.StatusOK)
		} else {
			// this just puts in something for the replacements to be able to make a request
			url := fmt.Sprintf("%sapi/v1%s", setting.AppURL, route.Pattern)
			for strings.Contains(url, "{") {
				before, after, _ := strings.Cut(url, "/{")
				_, after, _ = strings.Cut(after, "}/")
				url = fmt.Sprintf("%s/1/%s", before, after)
			}

			var req *RequestWrapper
			if strings.Contains(route.Pattern, "inbox") {
				req = NewRequestWithJSON(t, "POST", url, "{}")
			} else {
				req = NewRequest(t, "GET", url)
			}

			resp := MakeRequest(t, req, http.StatusBadRequest)
			assert.Contains(t, resp.Body.String(), "request signature verification failed")
		}
	}

	require.True(t, ranOne)
}
