// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/translation"
	"forgejo.org/tests"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPITwoFactor(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 16})

	req := NewRequest(t, "GET", "/api/v1/user").
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusOK)

	otpKey, err := totp.Generate(totp.GenerateOpts{
		SecretSize:  40,
		Issuer:      "gitea-test",
		AccountName: user.Name,
	})
	require.NoError(t, err)

	tfa := &auth_model.TwoFactor{
		UID: user.ID,
	}

	require.NoError(t, auth_model.NewTwoFactor(db.DefaultContext, tfa, otpKey.Secret()))

	req = NewRequest(t, "GET", "/api/v1/user").
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusUnauthorized)

	passcode, err := totp.GenerateCode(otpKey.Secret(), time.Now())
	require.NoError(t, err)

	req = NewRequest(t, "GET", "/api/v1/user").
		AddBasicAuth(user.Name)
	req.Header.Set("X-Gitea-OTP", passcode)
	MakeRequest(t, req, http.StatusOK)

	req = NewRequestf(t, "GET", "/api/v1/user").
		AddBasicAuth(user.Name)
	req.Header.Set("X-Forgejo-OTP", passcode)
	MakeRequest(t, req, http.StatusOK)
}

func TestAPIWebAuthn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 32})
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{UserID: user.ID})

	req := NewRequest(t, "GET", "/api/v1/user")
	req.SetBasicAuth(user.Name, "notpassword")

	resp := MakeRequest(t, req, http.StatusUnauthorized)

	type userResponse struct {
		Message string `json:"message"`
	}
	var userParsed userResponse

	DecodeJSON(t, resp, &userParsed)

	assert.Contains(t, userParsed.Message, "Basic authorization is not allowed while having security keys enrolled")
}

func TestAPIWithRequiredTwoFactor(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	type userResponse struct {
		Message string `json:"message"`
	}

	locale := translation.NewLocale("en-US")

	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	normalUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	inactiveUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})
	restrictedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})
	prohibitLoginUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 37})

	require2FaMessage := locale.TrString("error.must_enable_2fa", fmt.Sprintf("%suser/settings/security", setting.AppURL))
	const prohibitedMessage = "This account is prohibited from signing in, please contact your site administrator."
	const loginNotAllowedMessage = "user is not allowed login"

	runTest := func(t *testing.T, user *user_model.User, useTOTP bool, status int, messagePrefix string) {
		t.Helper()
		defer unittest.AssertSuccessfulDelete(t, &auth_model.TwoFactor{UID: user.ID})

		passcode := func() string {
			if !useTOTP {
				return ""
			}

			otpKey, err := totp.Generate(totp.GenerateOpts{
				SecretSize:  40,
				Issuer:      "forgejo-test",
				AccountName: user.Name,
			})
			require.NoError(t, err)

			require.NoError(t, auth_model.NewTwoFactor(t.Context(), &auth_model.TwoFactor{UID: user.ID}, otpKey.Secret()))

			passcode, err := totp.GenerateCode(otpKey.Secret(), time.Now())
			require.NoError(t, err)
			return passcode
		}()

		req := NewRequest(t, "GET", "/api/v1/user").
			AddBasicAuth(user.Name)

		if useTOTP {
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestf(t, "GET", "/api/v1/user").
				AddBasicAuth(user.Name)
			req.Header.Set("X-Forgejo-OTP", passcode)
		}

		resp := MakeRequest(t, req, status)

		if messagePrefix != "" {
			var response userResponse
			DecodeJSON(t, resp, &response)

			assert.True(t,
				slices.ContainsFunc(
					strings.Split(response.Message, "\n"),
					func(msg string) bool {
						return strings.HasPrefix(msg, messagePrefix)
					},
				),
				"expected prefix %q, but response message was %q", messagePrefix, response.Message)
		}
	}

	t.Run("NoneTwoFactorRequirement", func(t *testing.T) {
		// this should be the default, so don't have to set the variable

		t.Run("no 2fa", func(t *testing.T) {
			runTest(t, adminUser, false, http.StatusOK, "")
			runTest(t, normalUser, false, http.StatusOK, "")
			runTest(t, inactiveUser, false, http.StatusForbidden, prohibitedMessage)
			runTest(t, restrictedUser, false, http.StatusOK, "")
			runTest(t, prohibitLoginUser, false, http.StatusUnauthorized, loginNotAllowedMessage)
		})

		t.Run("enabled 2fa", func(t *testing.T) {
			runTest(t, adminUser, true, http.StatusOK, "")
			runTest(t, normalUser, true, http.StatusOK, "")
			runTest(t, inactiveUser, true, http.StatusForbidden, prohibitedMessage)
			runTest(t, restrictedUser, true, http.StatusOK, "")
			runTest(t, prohibitLoginUser, true, http.StatusUnauthorized, loginNotAllowedMessage)
		})
	})

	t.Run("AllTwoFactorRequirement", func(t *testing.T) {
		defer test.MockVariableValue(&setting.GlobalTwoFactorRequirement, setting.AllTwoFactorRequirement)()

		t.Run("no 2fa", func(t *testing.T) {
			runTest(t, adminUser, false, http.StatusForbidden, require2FaMessage)
			runTest(t, normalUser, false, http.StatusForbidden, require2FaMessage)
			runTest(t, inactiveUser, false, http.StatusForbidden, prohibitedMessage)
			runTest(t, restrictedUser, false, http.StatusForbidden, require2FaMessage)
			runTest(t, prohibitLoginUser, false, http.StatusUnauthorized, loginNotAllowedMessage)
		})

		t.Run("enabled 2fa", func(t *testing.T) {
			runTest(t, adminUser, true, http.StatusOK, "")
			runTest(t, normalUser, true, http.StatusOK, "")
			runTest(t, inactiveUser, true, http.StatusForbidden, prohibitedMessage)
			runTest(t, restrictedUser, true, http.StatusOK, "")
			runTest(t, prohibitLoginUser, true, http.StatusUnauthorized, loginNotAllowedMessage)
		})
	})

	t.Run("AdminTwoFactorRequirement", func(t *testing.T) {
		defer test.MockVariableValue(&setting.GlobalTwoFactorRequirement, setting.AdminTwoFactorRequirement)()

		t.Run("no 2fa", func(t *testing.T) {
			runTest(t, adminUser, false, http.StatusForbidden, require2FaMessage)
			runTest(t, normalUser, false, http.StatusOK, "")
			runTest(t, inactiveUser, false, http.StatusForbidden, prohibitedMessage)
			runTest(t, restrictedUser, false, http.StatusOK, "")
			runTest(t, prohibitLoginUser, false, http.StatusUnauthorized, loginNotAllowedMessage)
		})

		t.Run("enabled 2fa", func(t *testing.T) {
			runTest(t, adminUser, true, http.StatusOK, "")
			runTest(t, normalUser, true, http.StatusOK, "")
			runTest(t, inactiveUser, true, http.StatusForbidden, prohibitedMessage)
			runTest(t, restrictedUser, true, http.StatusOK, "")
			runTest(t, prohibitLoginUser, true, http.StatusUnauthorized, loginNotAllowedMessage)
		})
	})
}
