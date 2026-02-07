// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package shared

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/json"
	"forgejo.org/modules/jwtx"
	"forgejo.org/modules/test"
	"forgejo.org/modules/web"
	"forgejo.org/routers/common"
	"forgejo.org/services/auth/source/oauth2"
	"forgejo.org/services/authz"
	"forgejo.org/services/context"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReducer(t *testing.T) {
	defer unittest.OverrideFixtures("routers/api/shared/TestReducer")()
	require.NoError(t, unittest.PrepareTestDatabase())

	makeRecorder := func() *httptest.ResponseRecorder {
		buff := bytes.NewBufferString("")
		recorder := httptest.NewRecorder()
		recorder.Body = buff
		return recorder
	}

	r := web.NewRoute()
	r.Use(common.ProtocolMiddlewares()...)
	r.Use(Middlewares()...)

	type ReducerInfo struct {
		IsSigned         bool
		IsNil            bool
		IsAllAccess      bool
		IsPublicAccess   bool
		IsSpecificAccess bool
	}

	// mock oauth.Init() without the ctx
	var signingKey jwtx.SigningKey
	var verifier *jwtx.Verifier
	{
		secret := make([]byte, 32)
		signingCfg := jwtx.SigningKeyCfg{
			Algorithm:   "HS256",
			SecretBytes: &secret,
		}
		keyCfg := jwtx.KeyCfg{
			Signing: &signingCfg,
		}
		keyCfgP := &keyCfg
		var err error
		signingKey, verifier, err = jwtx.Init(&keyCfgP)
		require.NoError(t, err)
	}
	defer test.MockVariableValue(&oauth2.DefaultSigningKey, signingKey)()
	defer test.MockVariableValue(&oauth2.DefaultVerifier, verifier)()

	r.Get("/api/test", func(ctx *context.APIContext) {
		retval := ReducerInfo{
			IsSigned: ctx.IsSigned,
			IsNil:    ctx.Reducer == nil,
		}

		_, isAllAccess := ctx.Reducer.(*authz.AllAccessAuthorizationReducer)
		retval.IsAllAccess = isAllAccess

		_, isPublicAccess := ctx.Reducer.(*authz.PublicReposAuthorizationReducer)
		retval.IsPublicAccess = isPublicAccess

		_, isSpecificAccess := ctx.Reducer.(*authz.SpecificReposAuthorizationReducer)
		retval.IsSpecificAccess = isSpecificAccess

		ctx.JSON(http.StatusOK, retval)
	})

	t.Run("Basic Auth w/ PAT", func(t *testing.T) {
		t.Run("unrestricted access token", func(t *testing.T) {
			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.SetBasicAuth("token", "4a0c970da8bf58408a8c22264b2ac1ff47dadcce")
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.True(t, reducerInfo.IsAllAccess)
			assert.False(t, reducerInfo.IsPublicAccess)
			assert.False(t, reducerInfo.IsSpecificAccess)
		})

		t.Run("public-only access token", func(t *testing.T) {
			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.SetBasicAuth("token", "83909b5b978acc5620ae0c7b0e55b548da2e26b5")
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.False(t, reducerInfo.IsAllAccess)
			assert.True(t, reducerInfo.IsPublicAccess)
			assert.False(t, reducerInfo.IsSpecificAccess)
		})

		t.Run("specific-repo access token", func(t *testing.T) {
			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.SetBasicAuth("token", "46088605ec804b43ebd15cef1b3f210c31b066dd")
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.False(t, reducerInfo.IsAllAccess)
			assert.False(t, reducerInfo.IsPublicAccess)
			assert.True(t, reducerInfo.IsSpecificAccess)
		})
	})

	t.Run("Token Auth", func(t *testing.T) {
		t.Run("unrestricted access token", func(t *testing.T) {
			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.Header.Set("Authorization", "token 4a0c970da8bf58408a8c22264b2ac1ff47dadcce")
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.True(t, reducerInfo.IsAllAccess)
			assert.False(t, reducerInfo.IsPublicAccess)
			assert.False(t, reducerInfo.IsSpecificAccess)
		})

		t.Run("public-only access token", func(t *testing.T) {
			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.Header.Set("Authorization", "token 83909b5b978acc5620ae0c7b0e55b548da2e26b5")
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.False(t, reducerInfo.IsAllAccess)
			assert.True(t, reducerInfo.IsPublicAccess)
			assert.False(t, reducerInfo.IsSpecificAccess)
		})

		t.Run("specific-repo access token", func(t *testing.T) {
			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.Header.Set("Authorization", "token 46088605ec804b43ebd15cef1b3f210c31b066dd")
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.False(t, reducerInfo.IsAllAccess)
			assert.False(t, reducerInfo.IsPublicAccess)
			assert.True(t, reducerInfo.IsSpecificAccess)
		})
	})

	t.Run("OAuth", func(t *testing.T) {
		t.Run("unrestricted grant", func(t *testing.T) {
			grant := &auth_model.OAuth2Grant{
				UserID:        2,
				ApplicationID: 100, // fake, but required here for unique constraint
				Scope:         "write:repository",
			}
			_, err := db.GetEngine(t.Context()).Insert(grant)
			require.NoError(t, err)

			token := oauth2.Token{
				GrantID: grant.ID,
				Type:    oauth2.TypeAccessToken,
				Counter: 100,
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				},
			}
			signed, err := token.SignToken(oauth2.DefaultSigningKey)
			require.NoError(t, err)

			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.Header.Set("Authorization", fmt.Sprintf("bearer %s", signed))
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.True(t, reducerInfo.IsAllAccess)
			assert.False(t, reducerInfo.IsPublicAccess)
			assert.False(t, reducerInfo.IsSpecificAccess)
		})

		t.Run("public-only grant", func(t *testing.T) {
			grant := &auth_model.OAuth2Grant{
				UserID:        2,
				ApplicationID: 101, // fake, but required here for unique constraint
				Scope:         "write:repository public-only",
			}
			_, err := db.GetEngine(t.Context()).Insert(grant)
			require.NoError(t, err)

			token := oauth2.Token{
				GrantID: grant.ID,
				Type:    oauth2.TypeAccessToken,
				Counter: 100,
				RegisteredClaims: jwt.RegisteredClaims{
					IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				},
			}
			signed, err := token.SignToken(oauth2.DefaultSigningKey)
			require.NoError(t, err)

			recorder := makeRecorder()
			req, err := http.NewRequest("GET", "http://localhost:8000/api/test", nil)
			req.Header.Set("Authorization", fmt.Sprintf("bearer %s", signed))
			require.NoError(t, err)
			r.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusOK, recorder.Code)

			var reducerInfo ReducerInfo
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &reducerInfo))

			assert.True(t, reducerInfo.IsSigned)
			assert.False(t, reducerInfo.IsNil)
			assert.False(t, reducerInfo.IsAllAccess)
			assert.True(t, reducerInfo.IsPublicAccess)
			assert.False(t, reducerInfo.IsSpecificAccess)
		})
	})
}
