// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"forgejo.org/models/auth"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/json"
	"forgejo.org/modules/jwtx"
	"forgejo.org/modules/test"
	"forgejo.org/services/authz"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAuthorizedIntegration(t *testing.T) {
	ii := NewMockInternalIssuer(t)
	ii.On("IssuerPlaceholder").Return("urn:forgejo:authorized-issuer:internal:test2")
	RegisterInternalIssuerForTesting(t, "/fake-jwt-issuer", ii)

	makeValid := func() *auth.AuthorizedIntegration {
		return &auth.AuthorizedIntegration{
			Name:             "Test authorized integration",
			ResourceAllRepos: true,
			Scope:            auth.AccessTokenScopeReadRepository,
			UI:               auth.AuthorizedIntegrationUIGeneric,
			Issuer:           "urn:forgejo:authorized-issuer:internal:test2",
			ClaimRules:       &auth.ClaimRules{},
		}
	}

	t.Run("valid - all access", func(t *testing.T) {
		ai := makeValid()
		ai.ResourceAllRepos = true
		ai.Scope = auth.AccessTokenScopeReadRepository
		err := ValidateAuthorizedIntegration(ai, nil)
		require.NoError(t, err)
	})

	t.Run("valid - specified repos", func(t *testing.T) {
		ai := makeValid()
		ai.ResourceAllRepos = false
		ai.Scope = auth.AccessTokenScopeReadRepository
		resources := []*auth.AuthorizedIntegResourceRepo{{RepoID: 12}}
		err := ValidateAuthorizedIntegration(ai, resources)
		require.NoError(t, err)
	})

	t.Run("invalid - no specified repos", func(t *testing.T) {
		ai := makeValid()
		ai.ResourceAllRepos = false
		ai.Scope = auth.AccessTokenScopeReadRepository
		resources := []*auth.AuthorizedIntegResourceRepo{}
		err := ValidateAuthorizedIntegration(ai, resources)
		require.ErrorIs(t, err, authz.ErrSpecifiedReposNone)
	})

	t.Run("invalid - specified repos & public-only", func(t *testing.T) {
		ai := makeValid()
		ai.ResourceAllRepos = false
		ai.Scope = auth.AccessTokenScope(strings.Join([]string{string(auth.AccessTokenScopePublicOnly), string(auth.AccessTokenScopeReadRepository)}, ","))
		resources := []*auth.AuthorizedIntegResourceRepo{{RepoID: 12}}
		err := ValidateAuthorizedIntegration(ai, resources)
		require.ErrorIs(t, err, authz.ErrSpecifiedReposNoPublicOnly)
	})

	t.Run("invalid - specified repos unsupported scopes", func(t *testing.T) {
		ai := makeValid()
		ai.ResourceAllRepos = false
		ai.Scope = auth.AccessTokenScopeReadAdmin
		resources := []*auth.AuthorizedIntegResourceRepo{{RepoID: 12}}
		err := ValidateAuthorizedIntegration(ai, resources)
		require.ErrorIs(t, err, authz.ErrSpecifiedReposInvalidScope)
		require.ErrorContains(t, err, string(auth.AccessTokenScopeReadAdmin))
	})

	t.Run("invalid - missing UI", func(t *testing.T) {
		ai := makeValid()
		ai.UI = ""
		err := ValidateAuthorizedIntegration(ai, nil)
		require.ErrorIs(t, err, ErrAuthorizedIntegrationBadUI)
		require.ErrorContains(t, err, "invalid UI: \"\"")
	})

	t.Run("invalid - missing name", func(t *testing.T) {
		ai := makeValid()
		ai.Name = ""
		err := ValidateAuthorizedIntegration(ai, nil)
		var mfe *MissingFieldError
		require.ErrorAs(t, err, &mfe)
		assert.Equal(t, "Name", mfe.Field)
	})

	t.Run("invalid - checks external issuer name", func(t *testing.T) {
		ai := makeValid()
		ai.Issuer = "ftp://example.com/"
		err := ValidateAuthorizedIntegration(ai, nil)
		require.ErrorIs(t, err, ErrInvalidIssuer)
	})

	t.Run("invalid - checks claims issuer name", func(t *testing.T) {
		ai := makeValid()
		ai.ClaimRules = &auth.ClaimRules{Rules: []auth.ClaimRule{{}}}
		err := ValidateAuthorizedIntegration(ai, nil)
		require.ErrorIs(t, err, ErrInvalidClaimRules)
	})
}

func TestInsertAuthorizedIntegration(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ii := NewMockInternalIssuer(t)
	ii.On("IssuerPlaceholder").Return("urn:forgejo:authorized-issuer:internal:test3")
	RegisterInternalIssuerForTesting(t, "/fake-jwt-issuer", ii)

	t.Run("success inserts w/ repos", func(t *testing.T) {
		ai := &auth.AuthorizedIntegration{
			UserID:           2,
			UI:               auth.AuthorizedIntegrationUIGeneric,
			ResourceAllRepos: false,
			ClaimRules:       &auth.ClaimRules{},
			Name:             " Magical AI ",
			Scope:            auth.AccessTokenScopeReadRepository,
			Issuer:           "urn:forgejo:authorized-issuer:internal:test3",
		}
		rr := []*auth.AuthorizedIntegResourceRepo{
			{
				RepoID: 2,
			},
		}

		err := InsertAuthorizedIntegration(t.Context(), ai, rr)
		require.NoError(t, err)

		fromDB := unittest.AssertExistsAndLoadBean(t, &auth.AuthorizedIntegration{ID: ai.ID})
		assert.Equal(t, "Magical AI", fromDB.Name)

		// IntegID should have been initialized and the repo-specific record saved
		res := unittest.AssertExistsAndLoadBean(t, &auth.AuthorizedIntegResourceRepo{IntegID: ai.ID})
		assert.EqualValues(t, 2, res.RepoID)
	})

	t.Run("validates data", func(t *testing.T) {
		ai := &auth.AuthorizedIntegration{
			UserID:           2,
			UI:               auth.AuthorizedIntegrationUIGeneric,
			ResourceAllRepos: false,
			ClaimRules:       &auth.ClaimRules{},
			Name:             " Magical AI ",
			Issuer:           "urn:forgejo:authorized-issuer:internal:test3",
		}
		err := InsertAuthorizedIntegration(t.Context(), ai, nil)
		require.ErrorIs(t, err, authz.ErrSpecifiedReposNone)
	})
}

func TestUpdateAuthorizedIntegration(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ii := NewMockInternalIssuer(t)
	ii.On("IssuerPlaceholder").Return("urn:forgejo:authorized-issuer:internal:test4")
	RegisterInternalIssuerForTesting(t, "/fake-jwt-issuer", ii)

	prep := func(t *testing.T) (*auth.AuthorizedIntegration, []*auth.AuthorizedIntegResourceRepo) {
		ai := &auth.AuthorizedIntegration{
			UserID:           2,
			UI:               auth.AuthorizedIntegrationUIGeneric,
			ResourceAllRepos: false,
			ClaimRules:       &auth.ClaimRules{},
			Name:             " Magical AI ",
			Scope:            auth.AccessTokenScopeReadRepository,
			Issuer:           "urn:forgejo:authorized-issuer:internal:test4",
		}
		rr := []*auth.AuthorizedIntegResourceRepo{
			{
				RepoID: 2,
			},
		}
		err := InsertAuthorizedIntegration(t.Context(), ai, rr)
		require.NoError(t, err)
		return ai, rr
	}

	t.Run("update basic fields", func(t *testing.T) {
		ai, rr := prep(t)
		ai.Description = "This is the description field."

		err := UpdateAuthorizedIntegration(t.Context(), ai, rr)
		require.NoError(t, err)

		fromDB := unittest.AssertExistsAndLoadBean(t, &auth.AuthorizedIntegration{ID: ai.ID})
		assert.Equal(t, "Magical AI", fromDB.Name)
		assert.Equal(t, "This is the description field.", fromDB.Description)
		unittest.AssertCount(t, &auth.AuthorizedIntegResourceRepo{IntegID: ai.ID}, 1)
	})

	t.Run("update remove resource repos", func(t *testing.T) {
		ai, _ := prep(t)
		ai.ResourceAllRepos = true

		err := UpdateAuthorizedIntegration(t.Context(), ai, nil)
		require.NoError(t, err)

		unittest.AssertCount(t, &auth.AuthorizedIntegResourceRepo{IntegID: ai.ID}, 0)
	})

	t.Run("update add resource repos", func(t *testing.T) {
		ai, _ := prep(t)
		rr := []*auth.AuthorizedIntegResourceRepo{
			{
				RepoID: 2,
			},
			{
				RepoID: 3,
			},
		}

		err := UpdateAuthorizedIntegration(t.Context(), ai, rr)
		require.NoError(t, err)

		unittest.AssertCount(t, &auth.AuthorizedIntegResourceRepo{IntegID: ai.ID}, 2)
	})

	t.Run("validates data", func(t *testing.T) {
		ai, _ := prep(t)
		err := InsertAuthorizedIntegration(t.Context(), ai, nil)
		require.ErrorIs(t, err, authz.ErrSpecifiedReposNone)
	})
}

type ExternalIssuerTester struct {
	t               *testing.T
	jwtSigningKey   jwtx.SigningKey
	testServer      *httptest.Server
	resetHTTPClient func()
	tweaks          []tweak
	issuer          string
}

func newEITester(t *testing.T, tweaks ...tweak) *ExternalIssuerTester {
	eit := &ExternalIssuerTester{
		t:      t,
		tweaks: tweaks,
	}

	var jwtSigningKey jwtx.SigningKey
	var err error
	keyPath := filepath.Join(t.TempDir(), "jwt-rsa-2048.priv")
	jwtSigningKey, err = jwtx.InitAsymmetricSigningKey(keyPath, "RS256")
	require.NoError(t, err)
	eit.jwtSigningKey = jwtSigningKey

	eit.testServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/actions/.well-known/openid-configuration" {
			retval := &AuthorizedIntegrationOpenIDConfiguration{
				Issuer:                           eit.issuer,
				IDTokenSigningAlgValuesSupported: []string{"RS256"},
				JwksURI:                          fmt.Sprintf("%s/.keys", eit.issuer),
			}
			for _, tweak := range eit.tweaks {
				if tweak, is := tweak.(openIDTweak); is {
					tweak(retval)
				}
			}
			err := json.NewEncoder(w).Encode(retval)
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/api/actions/.keys" {
			jwk, err := eit.jwtSigningKey.ToJWK()
			require.NoError(t, err)
			jwk["use"] = "sig"
			jwkMapAny := make(map[string]any, len(jwk))
			for k, v := range jwk {
				jwkMapAny[k] = v // convert map[string]string -> map[string]any
			}
			retval := &AuthorizedIntegrationOpenIDKeys{
				Keys: []map[string]any{jwkMapAny},
			}
			for _, tweak := range eit.tweaks {
				if jwksTweak, is := tweak.(jwksTweak); is {
					jwksTweak(retval)
				}
			}
			_ = json.NewEncoder(w).Encode(retval) // no error checking -- some tests abort read
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	eit.issuer = fmt.Sprintf("%s/api/actions", eit.testServer.URL)

	// trust TLS cert of our mock client by inserting the test client for our test server into the global aiHTTPClient
	eit.resetHTTPClient = test.MockVariableValue(
		&GetAuthorizedIntegrationHTTPClient,
		func() *http.Client {
			return eit.testServer.Client()
		},
	)

	return eit
}

func (eit *ExternalIssuerTester) close() {
	eit.resetHTTPClient()
	eit.testServer.Close()
}

type tweak any

type openIDTweak func(*AuthorizedIntegrationOpenIDConfiguration)

type jwksTweak func(*AuthorizedIntegrationOpenIDKeys)

func TestValidateExternalIssuer(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		eit := newEITester(t)
		defer eit.close()
		err := validateExternalIssuer(eit.issuer)
		require.NoError(t, err)
	})

	t.Run("unparseable URL", func(t *testing.T) {
		err := validateExternalIssuer("hello? \x7f is this a URL?")
		require.ErrorIs(t, err, ErrInvalidIssuer)
		require.ErrorContains(t, err, "failed parsing issuer URL")
	})

	t.Run("404 OIDC", func(t *testing.T) {
		eit := newEITester(t)
		defer eit.close()
		err := validateExternalIssuer(eit.issuer + "/wrong-path")
		require.ErrorIs(t, err, ErrInvalidIssuer)
		require.ErrorContains(t, err, "non-OK response code: 404 Not Found")
	})

	t.Run("mismatched issuer", func(t *testing.T) {
		eit := newEITester(t,
			openIDTweak(func(oidc *AuthorizedIntegrationOpenIDConfiguration) {
				oidc.Issuer = "https://whoops.example.org"
			}))
		defer eit.close()
		err := validateExternalIssuer(eit.issuer)
		require.ErrorIs(t, err, ErrInvalidIssuer)
		require.ErrorContains(t, err, "has issuer \"https://whoops.example.org\", but input issuer was")
	})

	t.Run("no signing alg issuer", func(t *testing.T) {
		eit := newEITester(t,
			openIDTweak(func(oidc *AuthorizedIntegrationOpenIDConfiguration) {
				oidc.IDTokenSigningAlgValuesSupported = nil
			}))
		defer eit.close()
		err := validateExternalIssuer(eit.issuer)
		require.ErrorIs(t, err, ErrInvalidIssuer)
		require.ErrorContains(t, err, "lacks required field id_token_signing_alg_values_supported")
	})

	t.Run("no jwks_uri", func(t *testing.T) {
		eit := newEITester(t,
			openIDTweak(func(oidc *AuthorizedIntegrationOpenIDConfiguration) {
				oidc.JwksURI = ""
			}))
		defer eit.close()
		err := validateExternalIssuer(eit.issuer)
		require.ErrorIs(t, err, ErrInvalidIssuer)
		require.ErrorContains(t, err, "lacks required field jwks_uri")
	})

	t.Run("remote jwks_uri", func(t *testing.T) {
		eit := newEITester(t,
			openIDTweak(func(oidc *AuthorizedIntegrationOpenIDConfiguration) {
				oidc.JwksURI = "https://example.org/.keys"
			}))
		defer eit.close()
		err := validateExternalIssuer(eit.issuer)
		require.ErrorIs(t, err, ErrInvalidIssuer)
		require.ErrorContains(t, err, "jwks_uri host mismatch")
	})

	t.Run("empty JWKS", func(t *testing.T) {
		eit := newEITester(t,
			jwksTweak(func(keys *AuthorizedIntegrationOpenIDKeys) {
				keys.Keys = nil
			}))
		defer eit.close()
		err := validateExternalIssuer(eit.issuer)
		require.ErrorIs(t, err, ErrInvalidIssuer)
		require.ErrorContains(t, err, "had zero keys")
	})
}

func TestValidateClaimRules(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		err := validateClaimRules(nil, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim rules are nil")
	})

	t.Run("missing claim", func(t *testing.T) {
		err := validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: ""}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim is missing at root[0]")
	})

	t.Run("eq", func(t *testing.T) {
		err := validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimEqual, Value: ""}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim value missing at root[0].value")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimEqual, Value: "present"}},
		}, "root")
		require.NoError(t, err)
	})

	t.Run("glob", func(t *testing.T) {
		err := validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimGlob, Value: ""}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim value missing at root[0].value")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimGlob, Value: "abc["}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim glob invalid at root[0].value")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimGlob, Value: "pre*ent"}},
		}, "root")
		require.NoError(t, err)
	})

	t.Run("in", func(t *testing.T) {
		err := validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimIn, Values: nil}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim values missing at root[0].values")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimIn, Values: []string{}}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim values missing at root[0].values")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimIn, Values: []string{"1", "2"}}},
		}, "root")
		require.NoError(t, err)
	})

	t.Run("glob-in", func(t *testing.T) {
		err := validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimGlobIn, Values: nil}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim values missing at root[0].values")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimGlobIn, Values: []string{}}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim values missing at root[0].values")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimGlobIn, Values: []string{"abc", "abc["}}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim glob invalid at root[0].values[1]")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimGlobIn, Values: []string{"1", "2"}}},
		}, "root")
		require.NoError(t, err)
	})

	t.Run("nested", func(t *testing.T) {
		err := validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{{Claim: "c", Comparison: auth.ClaimNested}},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim rules are nil at root.c")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{
				{
					Claim:      "c",
					Comparison: auth.ClaimNested,
					Nested: &auth.ClaimRules{
						Rules: []auth.ClaimRule{
							{
								Claim:      "d",
								Comparison: auth.ClaimEqual,
							},
						},
					},
				},
			},
		}, "root")
		require.ErrorIs(t, err, ErrInvalidClaimRules)
		require.ErrorContains(t, err, "claim value missing at root.c[0].value")

		err = validateClaimRules(&auth.ClaimRules{
			Rules: []auth.ClaimRule{
				{
					Claim:      "c",
					Comparison: auth.ClaimNested,
					Nested: &auth.ClaimRules{
						Rules: []auth.ClaimRule{
							{
								Claim:      "d",
								Comparison: auth.ClaimEqual,
								Value:      "123",
							},
						},
					},
				},
			},
		}, "root")
		require.NoError(t, err)
	})
}
