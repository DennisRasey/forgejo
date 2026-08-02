// Copyright 2017 The Gogs Authors. All rights reserved.
// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/tests"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type makeRequestFunc func(testing.TB, *RequestWrapper, int) *httptest.ResponseRecorder

func TestGPGKeys(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	tokenWithGPGKeyScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	tt := []struct {
		name        string
		makeRequest makeRequestFunc
		token       string
		results     []int
	}{
		{
			name: "NoLogin", makeRequest: MakeRequest, token: "",
			results: []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
		},
		{
			name: "LoggedAsUser2", makeRequest: session.MakeRequest, token: token,
			results: []int{http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
		},
		{
			name: "LoggedAsUser2WithScope", makeRequest: session.MakeRequest, token: tokenWithGPGKeyScope,
			results: []int{http.StatusOK, http.StatusOK, http.StatusNotFound, http.StatusNoContent, http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusCreated, http.StatusNotFound, http.StatusCreated},
		},
	}

	for _, tc := range tt {
		// Basic test on result code
		t.Run(tc.name, func(t *testing.T) {
			t.Run("ViewOwnGPGKeys", func(t *testing.T) {
				testViewOwnGPGKeys(t, tc.makeRequest, tc.token, tc.results[0])
			})
			t.Run("ViewGPGKeys", func(t *testing.T) {
				testViewGPGKeys(t, tc.makeRequest, tc.token, tc.results[1])
			})
			t.Run("GetGPGKey", func(t *testing.T) {
				testGetGPGKey(t, tc.makeRequest, tc.token, tc.results[2])
			})
			t.Run("DeleteGPGKey", func(t *testing.T) {
				testDeleteGPGKey(t, tc.makeRequest, tc.token, tc.results[3])
			})

			t.Run("CreateInvalidGPGKey", func(t *testing.T) {
				testCreateInvalidGPGKey(t, tc.makeRequest, tc.token, tc.results[4])
			})
			t.Run("CreateNoneRegisteredEmailGPGKey", func(t *testing.T) {
				testCreateNoneRegisteredEmailGPGKey(t, tc.makeRequest, tc.token, tc.results[5])
			})
			t.Run("CreateValidGPGKey", func(t *testing.T) {
				testCreateValidGPGKey(t, tc.makeRequest, tc.token, tc.results[6])
			})
			t.Run("CreateValidSecondaryEmailGPGKeyNotActivated", func(t *testing.T) {
				testCreateValidSecondaryEmailGPGKey(t, tc.makeRequest, tc.token, tc.results[7])
			})
		})
	}

	// Check state after basic add
	t.Run("CheckState", func(t *testing.T) {
		var keys []*api.GPGKey

		req := NewRequest(t, "GET", "/api/v1/user/gpg_keys"). // GET all keys
									AddTokenAuth(tokenWithGPGKeyScope)
		resp := MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &keys)
		assert.Len(t, keys, 1)

		primaryKey1 := keys[0] // Primary key 1
		assert.Equal(t, "38EA3BCED732982C", primaryKey1.KeyID)
		assert.Len(t, primaryKey1.Emails, 1)
		assert.Equal(t, "user2@example.com", primaryKey1.Emails[0].Email)
		assert.True(t, primaryKey1.Emails[0].Verified)

		subKey := primaryKey1.SubsKey[0] // Subkey of 38EA3BCED732982C
		assert.Equal(t, "70D7C694D17D03AD", subKey.KeyID)
		assert.Empty(t, subKey.Emails)

		var key api.GPGKey
		req = NewRequest(t, "GET", "/api/v1/user/gpg_keys/"+strconv.FormatInt(primaryKey1.ID, 10)). // Primary key 1
														AddTokenAuth(tokenWithGPGKeyScope)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &key)
		assert.Equal(t, "38EA3BCED732982C", key.KeyID)
		assert.Len(t, key.Emails, 1)
		assert.Equal(t, "user2@example.com", key.Emails[0].Email)
		assert.True(t, key.Emails[0].Verified)

		req = NewRequest(t, "GET", "/api/v1/user/gpg_keys/"+strconv.FormatInt(subKey.ID, 10)). // Subkey of 38EA3BCED732982C
													AddTokenAuth(tokenWithGPGKeyScope)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &key)
		assert.Equal(t, "70D7C694D17D03AD", key.KeyID)
		assert.Empty(t, key.Emails)
	})

	// Check state after basic add
	t.Run("CheckCommits", func(t *testing.T) {
		t.Run("NotSigned", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/not-signed").
				AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.False(t, branch.Commit.Verification.Verified)
		})

		t.Run("SignedWithNotValidatedEmail", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/good-sign-not-yet-validated").
				AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.False(t, branch.Commit.Verification.Verified)
		})

		t.Run("SignedWithValidEmail", func(t *testing.T) {
			var branch api.Branch
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/branches/good-sign").
				AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			DecodeJSON(t, resp, &branch)
			assert.True(t, branch.Commit.Verification.Verified)
		})
	})
}

func testViewOwnGPGKeys(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "GET", "/api/v1/user/gpg_keys").
		AddTokenAuth(token)
	makeRequest(t, req, expected)
}

func testViewGPGKeys(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "GET", "/api/v1/users/user2/gpg_keys").
		AddTokenAuth(token)
	makeRequest(t, req, expected)
}

func testGetGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "GET", "/api/v1/user/gpg_keys/1").
		AddTokenAuth(token)
	makeRequest(t, req, expected)
}

func testDeleteGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	req := NewRequest(t, "DELETE", "/api/v1/user/gpg_keys/1").
		AddTokenAuth(token)
	makeRequest(t, req, expected)
}

func testCreateGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int, publicKey string) {
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/gpg_keys", api.CreateGPGKeyOption{
		ArmoredKey: publicKey,
	}).AddTokenAuth(token)
	makeRequest(t, req, expected)
}

func testCreateInvalidGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	testCreateGPGKey(t, makeRequest, token, expected, "invalid_key")
}

func testCreateNoneRegisteredEmailGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	testCreateGPGKey(t, makeRequest, token, expected, `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFmGUygBCACjCNbKvMGgp0fd5vyFW9olE1CLCSyyF9gQN2hSuzmZLuAZF2Kh
dCMCG2T1UwzUB/yWUFWJ2BtCwSjuaRv+cGohqEy6bhEBV90peGA33lHfjx7wP25O
7moAphDOTZtDj1AZfCh/PTcJut8Lc0eRDMhNyp/bYtO7SHNT1Hr6rrCV/xEtSAvR
3b148/tmIBiSadaLwc558KU3ucjnW5RVGins3AjBZ+TuT4XXVH/oeLSeXPSJ5rt1
rHwaseslMqZ4AbvwFLx5qn1OC9rEQv/F548QsA8m0IntLjoPon+6wcubA9Gra21c
Fp6aRYl9x7fiqXDLg8i3s2nKdV7+e6as6Tp9ABEBAAG0FG5vdGtub3duQGV4YW1w
bGUuY29tiQEcBBABAgAGBQJZhlMoAAoJEC8+pvYULDtte/wH/2JNrhmHwDY+hMj0
batIK4HICnkKxjIgbha80P2Ao08NkzSge58fsxiKDFYAQjHui+ZAw4dq79Ax9AOO
Iv2GS9+DUfWhrb6RF+vNuJldFzcI0rTW/z2q+XGKrUCwN3khJY5XngHfQQrdBtMK
qsoUXz/5B8g422RTbo/SdPsyYAV6HeLLeV3rdgjI1fpaW0seZKHeTXQb/HvNeuPg
qz+XV1g6Gdqa1RjDOaX7A8elVKxrYq3LBtc93FW+grBde8n7JL0zPM3DY+vJ0IJZ
INx/MmBfmtCq05FqNclvU+sj2R3N1JJOtBOjZrJHQbJhzoILou8AkxeX1A+q9OAz
1geiY5E=
=TkP3
-----END PGP PUBLIC KEY BLOCK-----`)
}

func testCreateValidGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	// User2 <user2@example.com> //primary & activated
	testCreateGPGKey(t, makeRequest, token, expected, `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFmGVsMBCACuxgZ7W7rI9xN08Y4M7B8yx/6/I4Slm94+wXf8YNRvAyqj30dW
VJhyBcnfNRDLKSQp5o/hhfDkCgdqBjLa1PnHlGS3PXJc0hP/FyYPD2BFvNMPpCYS
eu3T1qKSNXm6X0XOWD2LIrdiDC8HaI9FqZVMI/srMK2CF8XCL2m67W1FuoPlWzod
5ORy0IZB7spoF0xihmcgnEGElRmdo5w/vkGH8U7Zyn9Eb57UVFeafgeskf4wqB23
BjbMdW2YaB+yzMRwYgOnD5lnBD4uqSmvjaV9C0kxn7x+oJkkiRV8/z1cNcO+BaeQ
Akh/yTTeTzYGSc/ZOqCX1O+NOPgSeixVlqenABEBAAG0GVVzZXIyIDx1c2VyMkBl
eGFtcGxlLmNvbT6JAVQEEwEIAD4WIQRXgbSh0TtGbgRd7XI46jvO1zKYLAUCWYZW
wwIbAwUJA8JnAAULCQgHAgYVCAkKCwIEFgIDAQIeAQIXgAAKCRA46jvO1zKYLF/e
B/91wm2KLMIQBZBA9WA2/+9rQWTo9EqgYrXN60rEzX3cYJWXZiE4DrKR1oWDGNLi
KXOCW62snvJldolBqq0ZqaKvPKzl0Y5TRqbYEc9AjUSqgRin1b+G2DevLGT4ibq+
7ocQvz0XkASEUAgHahp0Ubiiib1521WwT/duL+AG8Gg0+DK09RfV3eX5/EOkQCKv
8cutqgsd2Smz40A8wXuJkRcipZBtrB/GkUaZ/eJdwEeSYZjEA9GWF61LJT2stvRN
HCk7C3z3pVEek1PluiFs/4VN8BG8yDzW4c0tLty4Fj3VwPqwIbB5AJbquVfhQCb4
Eep2lm3Lc9b1OwO5N3coPJkouQENBFmGVsMBCADAGba2L6NCOE1i3WIP6CPzbdOo
N3gdTfTgccAx9fNeon9jor+3tgEjlo9/6cXiRoksOV6W4wFab/ZwWgwN6JO4CGvZ
Wi7EQwMMMp1E36YTojKQJrcA9UvMnTHulqQQ88F5E845DhzFQM3erv42QZZMBAX3
kXCgy1GNFocl6tLUvJdEqs+VcJGGANMpmzE4WLa8KhSYnxipwuQ62JBy9R+cHyKT
OARk8znRqSu5bT3LtlrZ/HXu+6Oy4+2uCdNzZIh5J5tPS7CPA6ptl88iGVBte/CJ
7cjgJWSQqeYp2Y5QvsWAivkQ4Ww9plHbbwV0A2eaHsjjWzlUl3HoJ/snMOhBABEB
AAGJATwEGAEIACYWIQRXgbSh0TtGbgRd7XI46jvO1zKYLAUCWYZWwwIbDAUJA8Jn
AAAKCRA46jvO1zKYLBwLCACQOpeRVrwIKVaWcPMYjVHHJsGscaLKpgpARAUgbiG6
Cbc2WI8Sm3fRwrY0VAfN+u9QwrtvxANcyB3vTgTzw7FimfhOimxiTSO8HQCfjDZF
Xly8rq+Fua7+ClWUpy21IekW41VvZYjH2sL6EVP+UcEOaGAyN53XfhaRVZPhNtZN
NKAE9N5EG3rbsZ33LzJj40rEKlzFSseAAPft8qA3IXjzFBx+PQXHMpNCagL79he6
lqockTJ+oPmta4CF/J0U5LUr1tOZXheL3TP6m8d08gDrtn0YuGOPk87i9sJz+jR9
uy6MA3VSB99SK9ducGmE1Jv8mcziREroz2TEGr0zPs6h
=J59D
-----END PGP PUBLIC KEY BLOCK-----`)
}

func testCreateValidSecondaryEmailGPGKey(t *testing.T, makeRequest makeRequestFunc, token string, expected int) {
	// User2 <user2-2@example.com> //secondary and not activated
	testCreateGPGKey(t, makeRequest, token, expected, `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
6qgtsUdi5eviONRkBgeZtN3oxA==
=MgDv
-----END PGP PUBLIC KEY BLOCK-----`)
}

func TestAPIGPGMultipleKeys(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Token used for verification is valid for each minute, to prevent
	// this test being flaky wait till the next minute if we have less than
	// five seconds until the next minute.
	if _, _, sec := time.Now().Clock(); sec >= 55 {
		test.SleepTillNextMinute()
	}

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	publicKeyring := `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
=3agD
-----END PGP PUBLIC KEY BLOCK-----`

	privateKeyring := `-----BEGIN PGP PRIVATE KEY BLOCK-----

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
=9to/
-----END PGP PRIVATE KEY BLOCK-----
`
	block, err := armor.Decode(strings.NewReader(privateKeyring))
	require.NoError(t, err)
	keyring, err := openpgp.ReadKeyRing(block.Body)
	require.NoError(t, err)
	assert.Len(t, keyring, 1)

	var verificationToken string
	t.Run("No signature provided", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithJSON(t, "POST", "/api/v1/user/gpg_keys", api.CreateGPGKeyOption{
			ArmoredKey: publicKeyring,
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusNotFound)

		var apiError api.APIError
		DecodeJSON(t, resp, &apiError)

		var ok bool
		_, verificationToken, ok = strings.Cut(apiError.Message, ": ")
		assert.True(t, ok)
	})

	t.Run("Signature provided", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		signatureOutput := &bytes.Buffer{}
		require.NoError(t, openpgp.ArmoredDetachSign(signatureOutput, keyring[0], strings.NewReader(verificationToken), nil))

		req := NewRequestWithJSON(t, "POST", "/api/v1/user/gpg_keys", api.CreateGPGKeyOption{
			ArmoredKey: publicKeyring,
			Signature:  signatureOutput.String(),
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
	})

	t.Run("{user}.gpg", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/user2.gpg")
		resp := session.MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, `-----BEGIN PGP PUBLIC KEY BLOCK-----

xsFNBGlUhjYBEADHuyzOjbPk3U0qC85lGTecJBKb1CgUZ0xphv5wNg1TBzLjQLRB
DzbnhGAROTjS08dESwrG5sf8lNKRRr9rRIKQ7p+OqhK2AkOqGC25vB6vJSBhUYNy
UWDqV02XSmNy33FlwAgalIiff12hpZ+PctmzFcBY6TJ7Zh1dIKvS17CNZvkfjb/r
m1r8VQk8I4qdgVUg8Ky898T8B6/sAHhKcDZNXXoOLpCCUsm6STCVlp+YE0fN3H6u
UiyJ++zjrS85fzV9Ug+rDZr+V/ZiOc5e+mqIo51L9m9VfFsmZfJDZvPBcFJgcISv
S7LySW/lJvZ+ntKoFANzxzlLOUNYWJv+X/tYzJIoiu4J9NAuUwieFU85CYNbz9aO
7BUlVvNDVNn2sSTX/Aum9nqLvdi17pQRNmp7NFrmSzD9GbvD1G4C51tDesnCJI/j
m5QQpor8PeiOAcaHey/GFIS0+e1nytT3RU72P8M22u44QVp+WSAeahnsEBHVLbA7
BUoShykXul65d9Tko2H6lf4s5ggi66mXPvHH1O0Ry57JEcJVAFHpp+teckmXf8zD
a+/ha4U+RtJkkJ4RI6mPrFnamx57mYPT2ji0igpU2paAtIGhbDxSi5UD3ZycdmDj
Tw/uUTxl/lon+N7DVEFC+2F8HD2Aiw6TJ92AWs3CCZRsURMphH1W7cskvQARAQAB
zRZIYWhhIHVzZXIyQGV4YW1wbGUuY29twsGOBBMBCgA4FiEEr+MV6CFVMbWYwDdi
we3dkWyHwqYFAmlUhjYCGwMFCwkIBwIGFQoJCAsCBBYCAwECHgECF4AACgkQwe3d
kWyHwqapNhAAlPbcf96Ntyk0vttBOSDS/owq1fk2I/h66NgDK0G3601JT0JhFv1P
nY9YV5w5rzuTvCLbQWALr8UD52B8Ua6fEn5XjiLf6dU6OuZnYqIgTqi2qm4xCuNh
p5E1UqNLBb6Nv225R1CB6XGEda+wFQkTRif2e+bQOi4/+hl6tp6tQXkqAMdJAhD5
3v0nvvpExHkPtRyfnoP2pFLC3Oic1fVlNUD9Gz2U/rL+BkKVFTRXHfsdf1eP5osO
HbgXe8S8xWAW+K8+FeU/5uyzx6HVIMHvvu8ZxefEvkuFyBtzeikOulp7gY0OBSpr
ysRbPGUHXz2W+l3t21L3QKx++gq+aPW3ILkzyK0ovtXoJdU0QeJmbM7UaqfkYHcU
ut9HaohCheLIcWlpTaqJlqMeSxx+I0NHIbRGILM2Db0DWRms0zZxYIywtlJlolSP
hJeq3jd/Yc334CXVtY/bU8pxInJewzUtAqMhJ8AUvUfsUpmaMUg0Vkqv7NrZBhL7
TFZXsVpw1M3Vc5nF8cfX8GfDh16cmFx9Tpz9SrVPSVAwiK1V+Lp9p6CGPZhE3Y33
b94YUmt+CvWxKx8OeiUGEqLG3W/KpXbnYUUetyCtCMikdmaGPpvq0ifDv5AEt3eZ
DWcqZOZ1dSTG9wmKghkVrgnB6Zow4c62WFtrDtFoN0XAjC42KsufgNQ=
=766/
-----END PGP PUBLIC KEY BLOCK-----`, resp.Body.String())
	})
}
