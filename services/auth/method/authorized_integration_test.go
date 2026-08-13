// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package method

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	"forgejo.org/modules/cache"
	"forgejo.org/modules/json"
	"forgejo.org/modules/jwtx"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"
	"forgejo.org/services/auth"

	mc "code.forgejo.org/go-chi/cache"
	"github.com/golang-jwt/jwt/v5"
	gouuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCheckClaims(t *testing.T) {
	ai := &AuthorizedIntegration{}
	rules := func(rule ...auth_model.ClaimRule) *auth_model.ClaimRules {
		return &auth_model.ClaimRules{Rules: rule}
	}
	eq := func(claim, value string) auth_model.ClaimRule {
		return auth_model.ClaimRule{
			Claim:      claim,
			Comparison: auth_model.ClaimEqual,
			Value:      value,
		}
	}
	in := func(claim string, values []string) auth_model.ClaimRule {
		return auth_model.ClaimRule{
			Claim:      claim,
			Comparison: auth_model.ClaimIn,
			Values:     values,
		}
	}
	glob := func(claim, value string) auth_model.ClaimRule {
		return auth_model.ClaimRule{
			Claim:      claim,
			Comparison: auth_model.ClaimGlob,
			Value:      value,
		}
	}
	globIn := func(claim string, values []string) auth_model.ClaimRule {
		return auth_model.ClaimRule{
			Claim:      claim,
			Comparison: auth_model.ClaimGlobIn,
			Values:     values,
		}
	}
	nest := func(claim string, inner ...auth_model.ClaimRule) auth_model.ClaimRule {
		return auth_model.ClaimRule{
			Claim:      claim,
			Comparison: auth_model.ClaimNested,
			Nested:     rules(inner...),
		}
	}

	t.Run("nil claims", func(t *testing.T) {
		require.NoError(t, ai.checkClaims(map[string]any{}, nil))
	})

	t.Run("flexibleClaims's fixed and other fields", func(t *testing.T) {
		t.Run("iss", func(t *testing.T) {
			c := &flexibleClaims{}
			rules := rules(eq("iss", "https://example.org"))

			c.Issuer = "https://example.org"
			require.NoError(t, ai.checkClaims(c, rules))

			c.Issuer = "https://other.example.org"
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"iss\" must be \"https://example.org\", but was \"https://other.example.org\"")
		})

		t.Run("sub", func(t *testing.T) {
			c := &flexibleClaims{}
			rules := rules(eq("sub", "my-stuff"))

			c.Subject = "my-stuff"
			require.NoError(t, ai.checkClaims(c, rules))

			c.Subject = "my-other-stuff"
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"sub\" must be \"my-stuff\", but was \"my-other-stuff\"")
		})

		t.Run("jti", func(t *testing.T) {
			c := &flexibleClaims{}
			rules := rules(eq("jti", "7d9a2e85-6b8d-4b59-bca0-09d702476338"))

			c.ID = "7d9a2e85-6b8d-4b59-bca0-09d702476338"
			require.NoError(t, ai.checkClaims(c, rules))

			c.ID = "8855d16c-cd5f-4ace-b626-5c5875e1a993"
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"jti\" must be \"7d9a2e85-6b8d-4b59-bca0-09d702476338\", but was \"8855d16c-cd5f-4ace-b626-5c5875e1a993\"")
		})

		t.Run("aud", func(t *testing.T) {
			c := &flexibleClaims{}
			rules := rules(eq("aud", "the-best-audience"))

			c.Audience = jwt.ClaimStrings{"the-best-audience"}
			require.NoError(t, ai.checkClaims(c, rules))

			c.Audience = jwt.ClaimStrings{"something-else"}
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"aud\" must be \"the-best-audience\", but was \"something-else\"")

			c.Audience = jwt.ClaimStrings{"aud1", "aud2"}
			require.ErrorContains(t, ai.checkClaims(c, rules), "required one and only one `aud` claim, but received 2")
		})

		t.Run("arbitrary field", func(t *testing.T) {
			c := &flexibleClaims{other: map[string]any{}}
			rules := rules(eq("arbitrary", "abc"))

			c.other["arbitrary"] = "abc"
			require.NoError(t, ai.checkClaims(c, rules))

			c.other["arbitrary"] = "123"
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must be \"abc\", but was \"123\"")

			delete(c.other, "arbitrary")
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim rule on \"arbitrary\" couldn't be satisfied: claim not found")
		})
	})

	t.Run("map[string]any input", func(t *testing.T) {
		t.Run("arbitrary field", func(t *testing.T) {
			c := map[string]any{}
			rules := rules(eq("arbitrary", "abc"))

			c["arbitrary"] = "abc"
			require.NoError(t, ai.checkClaims(c, rules))

			c["arbitrary"] = "123"
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must be \"abc\", but was \"123\"")

			delete(c, "arbitrary")
			require.ErrorContains(t, ai.checkClaims(c, rules), "claim rule on \"arbitrary\" couldn't be satisfied: claim not found")
		})
	})

	t.Run("unexpected input", func(t *testing.T) {
		c := map[string]int{}
		rules := rules(eq("arbitrary", "abc"))
		c["arbitrary"] = 123
		require.ErrorContains(t, ai.checkClaims(c, rules), "unexpected incoming claims type: map[string]int")
	})

	t.Run("comparison ClaimEqual", func(t *testing.T) {
		c := map[string]any{}
		rules := rules(eq("arbitrary", "abc"))

		c["arbitrary"] = "abc"
		require.NoError(t, ai.checkClaims(c, rules))

		c["arbitrary"] = "123"
		require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must be \"abc\", but was \"123\"")

		c["arbitrary"] = 123
		require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must be a string, but was int")
	})

	t.Run("comparison ClaimIn", func(t *testing.T) {
		c := map[string]any{}
		rules := rules(in("arbitrary", []string{"abc", "def"}))

		c["arbitrary"] = "abc"
		require.NoError(t, ai.checkClaims(c, rules))
		c["arbitrary"] = "def"
		require.NoError(t, ai.checkClaims(c, rules))

		c["arbitrary"] = "123"
		require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must be one of [\"abc\" \"def\"], but was \"123\"")

		c["arbitrary"] = 123
		require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must be a string, but was int")
	})

	t.Run("comparison ClaimIn empty", func(t *testing.T) {
		c := map[string]any{}
		rules := rules(in("arbitrary", []string{}))

		require.ErrorContains(t, ai.checkClaims(c, rules), "claim rule on \"arbitrary\" couldn't be satisfied: claim not found")

		c["arbitrary"] = "abc"
		require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must be one of [], but was \"abc\"")
	})

	t.Run("comparison ClaimGlob", func(t *testing.T) {
		c := map[string]any{}
		r := rules(glob("arbitrary", "*c"))

		c["arbitrary"] = "abc"
		require.NoError(t, ai.checkClaims(c, r))

		c["arbitrary"] = "123"
		require.ErrorContains(t, ai.checkClaims(c, r), "claim \"arbitrary\" must match glob \"*c\", but value \"123\" did not match")

		c["arbitrary"] = "this string contains a c or two but doesn't end with one" // ensure glob isn't OK w/ a partial match
		require.ErrorContains(t, ai.checkClaims(c, r), "claim \"arbitrary\" must match glob \"*c\", but value \"this string contains a c or two but doesn't end with one\" did not match")

		c["arbitrary"] = 123
		require.ErrorContains(t, ai.checkClaims(c, r), "claim \"arbitrary\" must be a string, but was int")

		r = rules(glob("arbitrary", "[abc"))
		c["arbitrary"] = "abc"
		require.ErrorContains(t, ai.checkClaims(c, r), "unable to parse glob for claim rule on \"arbitrary\"; glob = \"[abc\", err = unexpected end of input")
	})

	t.Run("comparison ClaimGlobIn", func(t *testing.T) {
		c := map[string]any{}
		r := rules(globIn("arbitrary", []string{"*c", "*def*"}))

		c["arbitrary"] = "abc"
		require.NoError(t, ai.checkClaims(c, r))
		c["arbitrary"] = "abcdef"
		require.NoError(t, ai.checkClaims(c, r))

		c["arbitrary"] = "123"
		require.ErrorContains(t, ai.checkClaims(c, r), "claim \"arbitrary\" must glob match one of [\"*c\" \"*def*\"], but value \"123\" did not match")

		c["arbitrary"] = "this string contains a c or two but doesn't end with one" // ensure glob isn't OK w/ a partial match
		require.ErrorContains(t, ai.checkClaims(c, r), "claim \"arbitrary\" must glob match one of [\"*c\" \"*def*\"], but value \"this string contains a c or two but doesn't end with one\" did not match")

		c["arbitrary"] = 123
		require.ErrorContains(t, ai.checkClaims(c, r), "claim \"arbitrary\" must be a string, but was int")

		r = rules(globIn("arbitrary", []string{"[abc"}))
		c["arbitrary"] = "abc"
		require.ErrorContains(t, ai.checkClaims(c, r), "unable to parse glob for claim rule on \"arbitrary\"; glob = \"[abc\", err = unexpected end of input")
	})

	t.Run("comparison ClaimGlobIn empty", func(t *testing.T) {
		c := map[string]any{}
		rules := rules(globIn("arbitrary", []string{}))

		require.ErrorContains(t, ai.checkClaims(c, rules), "claim rule on \"arbitrary\" couldn't be satisfied: claim not found")

		c["arbitrary"] = "abc"
		require.ErrorContains(t, ai.checkClaims(c, rules), "claim \"arbitrary\" must glob match one of [], but value \"abc\" did not match")
	})

	t.Run("comparison ClaimNested", func(t *testing.T) {
		c := map[string]any{}
		r := rules(nest("nest", eq("arbitrary", "abc")))

		c["nest"] = map[string]any{"arbitrary": "abc"}
		require.NoError(t, ai.checkClaims(c, r))

		c["nest"] = map[string]any{"blah": "abc"}
		require.ErrorContains(t, ai.checkClaims(c, r), "in nested claim \"nest\": claim rule on \"arbitrary\" couldn't be satisfied: claim not found")

		c["nest"] = map[string]int{"blah": 123}
		require.ErrorContains(t, ai.checkClaims(c, r), "claim \"nest\" must be a map, but was map[string]int")
	})

	t.Run("multiple rules", func(t *testing.T) {
		c := map[string]any{
			"arb1": "abc",
			"arb2": "123",
			"arb3": "def",
		}
		rules := rules(
			eq("arb1", "abc"),
			eq("arb2", "123"),
		)

		require.NoError(t, ai.checkClaims(c, rules))

		delete(c, "arb1")
		require.ErrorContains(t, ai.checkClaims(c, rules), "\"arb1\"")

		c["arb1"] = "abc"
		delete(c, "arb2")
		require.ErrorContains(t, ai.checkClaims(c, rules), "\"arb2\"")
	})
}

func requireOutput[K auth.MethodOutput](t *testing.T, o auth.MethodOutput) K {
	t.Helper()
	k, isType := o.(K)
	require.True(t, isType, "expected Verify output to be type %T, but was %T: %v", *new(K), o, o)
	return k
}

func TestAuthorizedIntegration(t *testing.T) {
	t.Run("no token", func(t *testing.T) {
		ai := &AuthorizedIntegration{}
		aiBasic := &AuthorizedIntegration{PermitBasic: true}
		req := httptest.NewRequest("GET", "https://example.org", nil)
		output := ai.Verify(req, nil, nil)
		requireOutput[*auth.AuthenticationNotAttempted](t, output)
		output = aiBasic.Verify(req, nil, nil)
		requireOutput[*auth.AuthenticationNotAttempted](t, output)
	})

	t.Run("not a JWT", func(t *testing.T) {
		ai := &AuthorizedIntegration{}
		req := httptest.NewRequest("GET", "https://example.org", nil)
		req.Header.Set("Authorization", "Bearer abc")
		output := ai.Verify(req, nil, nil)
		err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
		require.ErrorContains(t, err, "parse JWT error")
	})

	t.Run("valid Bearer JWT", func(t *testing.T) {
		ait := newAITester(t)
		defer ait.close()
		output := ait.bearerRequest()
		success := requireOutput[*auth.AuthenticationSuccess](t, output)
		res := success.Result
		assert.EqualValues(t, 2, res.User().ID)
		hasScope, scope := res.Scope().Get()
		assert.True(t, hasScope)
		assert.Equal(t, auth_model.AccessTokenScopeAll, scope)
		assert.NotNil(t, res.Reducer())
		hasExpiry, _ := res.ExpiresAt().Get()
		assert.False(t, hasExpiry)
	})

	t.Run("valid Basic JWT", func(t *testing.T) {
		t.Run("PermitBasic", func(t *testing.T) {
			ait := newAITester(t,
				aiTweak(func(ai *AuthorizedIntegration) {
					ai.PermitBasic = true
				}))
			defer ait.close()
			output := ait.basicRequest()
			requireOutput[*auth.AuthenticationSuccess](t, output)
		})

		t.Run("!PermitBasic", func(t *testing.T) {
			ait := newAITester(t,
				aiTweak(func(ai *AuthorizedIntegration) {
					ai.PermitBasic = false
				}))
			defer ait.close()
			output := ait.basicRequest()
			requireOutput[*auth.AuthenticationNotAttempted](t, output)
		})
	})

	t.Run("JWT expiry", func(t *testing.T) {
		t.Run("JWT expired", func(t *testing.T) {
			ait := newAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.ExpiresAt = jwt.NewNumericDate(time.Date(2026, time.January, 1, 12, 0, 0, 0, time.Local))
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "token is expired")
		})

		t.Run("JWT will expire", func(t *testing.T) {
			ait := newAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.ExpiresAt = jwt.NewNumericDate(time.Date(2026, time.January, 1, 20, 0, 0, 0, time.UTC))
				}))
			defer ait.close()
			output := ait.bearerRequest()
			success := requireOutput[*auth.AuthenticationSuccess](t, output)
			res := success.Result
			hasExpiry, expiry := res.ExpiresAt().Get()
			assert.True(t, hasExpiry)
			assert.Equal(t, timeutil.TimeStamp(1767297600), expiry)
		})
	})

	t.Run("JWT issued at", func(t *testing.T) {
		ait := newAITester(t,
			claimTweak(func(rc *flexibleClaims) {
				rc.IssuedAt = jwt.NewNumericDate(time.Date(2027, time.January, 1, 12, 0, 0, 0, time.Local))
			}))
		defer ait.close()
		output := ait.bearerRequest()
		err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
		require.ErrorContains(t, err, "token used before issued")
	})

	t.Run("JWT not before", func(t *testing.T) {
		ait := newAITester(t,
			claimTweak(func(rc *flexibleClaims) {
				rc.NotBefore = jwt.NewNumericDate(time.Date(2027, time.January, 1, 12, 0, 0, 0, time.Local))
			}))
		defer ait.close()
		output := ait.bearerRequest()
		err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
		require.ErrorContains(t, err, "token is not valid yet")
	})

	t.Run("issuer", func(t *testing.T) {
		t.Run("missing in claim", func(t *testing.T) {
			ait := newAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.Issuer = ""
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "invalid `iss` claim")
		})

		t.Run("mismatch DB", func(t *testing.T) {
			ait := newAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.Issuer = "https://whoops.example.org"
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "matching authorized_integration not found")
		})

		t.Run("mismatch openid metadata", func(t *testing.T) {
			ait := newAITester(t,
				openIDTweak(func(oidc *auth.AuthorizedIntegrationOpenIDConfiguration, _ *AuthorizedIntegrationTester) {
					oidc.Issuer = "https://whoops.example.org"
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "issuer mismatch")
		})

		t.Run("non-HTTPS issuer", func(t *testing.T) {
			ait := newAITester(t,
				aiDBTweak(func(aiDB *auth_model.AuthorizedIntegration) {
					aiDB.Issuer = "http://whoops.example.org"
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "unsupported URL scheme: \"http://")
		})

		t.Run("signing alg values supported doesn't include in-use alg", func(t *testing.T) {
			ait := newAITester(t,
				openIDTweak(func(oidc *auth.AuthorizedIntegrationOpenIDConfiguration, _ *AuthorizedIntegrationTester) {
					oidc.IDTokenSigningAlgValuesSupported = []string{"WEIRD"}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, " issuer supports signature algorithms []string{\"WEIRD\"}, but received token with algorithm RS256")
		})
	})

	t.Run("audience", func(t *testing.T) {
		t.Run("missing in claim", func(t *testing.T) {
			ait := newAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.Audience = jwt.ClaimStrings{}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "required one and only one `aud` claim, but received 0")
		})

		t.Run("multiple in claim", func(t *testing.T) {
			ait := newAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.Audience = jwt.ClaimStrings{"abc", "def"}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "required one and only one `aud` claim, but received 2")
		})

		t.Run("mismatch DB", func(t *testing.T) {
			ait := newAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.Audience = jwt.ClaimStrings{"abc"}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "matching authorized_integration not found")
		})
	})

	t.Run("checks claim rules", func(t *testing.T) {
		ait := newAITester(t,
			claimTweak(func(rc *flexibleClaims) {
				rc.other["custom-claim"] = "oops wrong claim"
			}))
		defer ait.close()
		output := ait.bearerRequest()
		err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
		require.ErrorContains(t, err, "claim \"custom-claim\" must be \"custom-claim-value\"")
	})

	t.Run("key algorithms", func(t *testing.T) {
		for _, alg := range jwtx.ValidAsymmetricAlgorithms {
			t.Run(alg, func(t *testing.T) {
				ait := newAITester(t,
					jwtxKeyTweak(func() jwtx.SigningKey {
						keyPath := filepath.Join(t.TempDir(), fmt.Sprintf("jwt-%s.priv", alg))
						jwtSigningKey, err := jwtx.InitAsymmetricSigningKey(keyPath, alg)
						require.NoError(t, err)
						return jwtSigningKey
					}),
					openIDTweak(func(oidc *auth.AuthorizedIntegrationOpenIDConfiguration, _ *AuthorizedIntegrationTester) {
						oidc.IDTokenSigningAlgValuesSupported = []string{alg}
					}),
				)
				defer ait.close()
				output := ait.bearerRequest()
				requireOutput[*auth.AuthenticationSuccess](t, output)
			})
		}
	})

	t.Run("JWKS", func(t *testing.T) {
		t.Run("jwks_uri host mismatch", func(t *testing.T) {
			ait := newAITester(t,
				openIDTweak(func(oidc *auth.AuthorizedIntegrationOpenIDConfiguration, ait *AuthorizedIntegrationTester) {
					oidc.JwksURI = "https://whoops.example.org/.keys"
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "jwks_uri host mismatch: must be the same as issuer host")
		})

		t.Run("non-HTTPS JWKS address", func(t *testing.T) {
			ait := newAITester(t,
				openIDTweak(func(oidc *auth.AuthorizedIntegrationOpenIDConfiguration, ait *AuthorizedIntegrationTester) {
					oidc.JwksURI = strings.ReplaceAll(ait.testServer.URL, "https://", "http://")
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "unsupported URL scheme: \"http://")
		})

		t.Run("missing key", func(t *testing.T) {
			ait := newAITester(t,
				jwksTweak(func(keys *auth.AuthorizedIntegrationOpenIDKeys) {
					keys.Keys = []map[string]any{}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "no key identified")
		})

		t.Run("alg missing", func(t *testing.T) {
			ait := newAITester(t,
				jwksTweak(func(keys *auth.AuthorizedIntegrationOpenIDKeys) {
					for k := range keys.Keys {
						delete(keys.Keys[k], "alg")
					}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			// per RFC7517 "alg" is optional
			requireOutput[*auth.AuthenticationSuccess](t, output)
		})

		t.Run("alg mismatch", func(t *testing.T) {
			ait := newAITester(t,
				jwksTweak(func(keys *auth.AuthorizedIntegrationOpenIDKeys) {
					for k := range keys.Keys {
						keys.Keys[k]["alg"] = "WEIRD"
					}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "doesn't match expected algorithm RS256, was WEIRD")
		})

		t.Run("use missing", func(t *testing.T) {
			ait := newAITester(t,
				jwksTweak(func(keys *auth.AuthorizedIntegrationOpenIDKeys) {
					for k := range keys.Keys {
						delete(keys.Keys[k], "use")
					}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			// per RFC7517 "use" is optional
			requireOutput[*auth.AuthenticationSuccess](t, output)
		})

		t.Run("use isn't 'sig'", func(t *testing.T) {
			ait := newAITester(t,
				jwksTweak(func(keys *auth.AuthorizedIntegrationOpenIDKeys) {
					for k := range keys.Keys {
						keys.Keys[k]["use"] = "enc"
					}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "isn't designated for signing usage, was enc")
		})

		t.Run("large JWKS document", func(t *testing.T) {
			ait := newAITester(t,
				jwksTweak(func(keys *auth.AuthorizedIntegrationOpenIDKeys) {
					var keyContents map[string]any
					for _, v := range keys.Keys {
						keyContents = v
					}
					for range 128 {
						keys.Keys = append(keys.Keys, keyContents)
					}
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "failed to decode (response body restricted to 16384 bytes)")
		})
	})

	t.Run("specific scopes", func(t *testing.T) {
		ait := newAITester(t,
			aiDBTweak(func(aiDB *auth_model.AuthorizedIntegration) {
				aiDB.Scope = "read:repository,read:user"
			}))
		defer ait.close()
		output := ait.bearerRequest()
		success := requireOutput[*auth.AuthenticationSuccess](t, output)
		res := success.Result
		hasScope, scope := res.Scope().Get()
		assert.True(t, hasScope)
		readRepository, err := scope.HasScope(auth_model.AccessTokenScopeReadRepository)
		require.NoError(t, err)
		assert.True(t, readRepository, "read:repository")
		readUser, err := scope.HasScope(auth_model.AccessTokenScopeReadUser)
		require.NoError(t, err)
		assert.True(t, readUser, "read:user")
		writeAdmin, err := scope.HasScope(auth_model.AccessTokenScopeWriteAdmin)
		require.NoError(t, err)
		assert.False(t, writeAdmin, "write:admin")
	})

	t.Run("cache", func(t *testing.T) {
		t.Run("miss and store", func(t *testing.T) {
			c := cache.NewMockCache(t)
			defer test.MockVariableValue(&auth.GetAuthorizedIntegrationCache, func() mc.Cache { return c })()
			defer test.MockVariableValue(&setting.AuthorizedIntegration.CacheTTL, 10*time.Minute)()

			var cacheKey string
			c.On("Get", mock.AnythingOfType("string")).
				Run(func(args mock.Arguments) {
					key := args.Get(0).(string)
					assert.True(t, strings.HasPrefix(key, "auth-int-remote:https://"), "key %s should have key prefix", key)
					cacheKey = key
				}).Return(nil)
			c.On("Put", mock.Anything, mock.Anything, mock.Anything).
				Once().
				Run(func(args mock.Arguments) {
					putKey := args.Get(0).(string)
					assert.Equal(t, cacheKey, putKey)
					putContents := args.Get(1).([]byte)
					assert.Contains(t, string(putContents), "\"issuer\":")
					assert.Contains(t, string(putContents), "\"jwks_uri\":")
					assert.EqualValues(t, 600, args.Get(2))
				}).Return(nil)
			c.On("Put", mock.Anything, mock.Anything, mock.Anything).
				Once().
				Run(func(args mock.Arguments) {
					putKey := args.Get(0).(string)
					assert.Equal(t, cacheKey, putKey)
					putContents := args.Get(1).([]byte)
					assert.Contains(t, string(putContents), "\"alg\":\"RS256\"")
					assert.Contains(t, string(putContents), "\"kty\":\"RSA\"")
					assert.EqualValues(t, 600, args.Get(2))
				}).Return(nil)

			ait := newAITester(t)
			defer ait.close()
			output := ait.bearerRequest()
			requireOutput[*auth.AuthenticationSuccess](t, output)
		})

		t.Run("hit", func(t *testing.T) {
			var oidcMetadata []byte
			var jwksData []byte
			ait := newAITester(t,
				openIDTweak(func(oi *auth.AuthorizedIntegrationOpenIDConfiguration, ait *AuthorizedIntegrationTester) {
					var err error
					oidcMetadata, err = json.Marshal(oi)
					require.NoError(t, err)
				}),
				jwksTweak(func(oi *auth.AuthorizedIntegrationOpenIDKeys) {
					var err error
					jwksData, err = json.Marshal(oi)
					require.NoError(t, err)
				}),
			)
			defer ait.close()
			ait.bearerRequest() // populate oidcMetadata & jwksData by making a request

			t.Run("cache returns []byte", func(t *testing.T) {
				c := cache.NewMockCache(t)
				defer test.MockVariableValue(&auth.GetAuthorizedIntegrationCache, func() mc.Cache { return c })()

				c.On("Get",
					mock.MatchedBy(func(key string) bool {
						return strings.Contains(key, ".well-known/openid-configuration")
					})).
					Return(oidcMetadata)
				c.On("Get",
					mock.MatchedBy(func(key string) bool {
						return strings.Contains(key, ".keys")
					})).
					Return(jwksData)

				ait.bearerRequest()
			})

			t.Run("cache returns string", func(t *testing.T) {
				c := cache.NewMockCache(t)
				defer test.MockVariableValue(&auth.GetAuthorizedIntegrationCache, func() mc.Cache { return c })()

				c.On("Get",
					mock.MatchedBy(func(key string) bool {
						return strings.Contains(key, ".well-known/openid-configuration")
					})).
					Return(string(oidcMetadata))
				c.On("Get",
					mock.MatchedBy(func(key string) bool {
						return strings.Contains(key, ".keys")
					})).
					Return(string(jwksData))

				ait.bearerRequest()
			})
		})
	})

	t.Run("internal issuer", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ait := newInternalIssuerAITester(t)
			defer ait.close()
			output := ait.bearerRequest()
			requireOutput[*auth.AuthenticationSuccess](t, output)
		})

		t.Run("mismatched issuer app URL", func(t *testing.T) {
			ait := newInternalIssuerAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.Issuer = "https://example.org/fake-jwt-issuer" // correct suffix, incorrect prefix
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "matching authorized_integration not found")
			ait.ii.ExpectedCalls = nil // InternalIssuer should have zero calls
		})

		t.Run("mismatched issuer URL suffix", func(t *testing.T) {
			ait := newInternalIssuerAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.Issuer = setting.AppURL + "/fake-jwt-issuer-123" // correct prefix, incorrect suffix
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "matching authorized_integration not found")
			ait.ii.ExpectedCalls = nil // InternalIssuer should have zero calls
		})

		t.Run("mismatched DB issuer placeholder", func(t *testing.T) {
			ait := newInternalIssuerAITester(t,
				aiDBTweak(func(ai *auth_model.AuthorizedIntegration) {
					ai.Issuer = "urn:forgejo:authorized-issuer:internal:bad-choice-here"
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "matching authorized_integration not found")
			ait.ii.ExpectedCalls = nil // InternalIssuer should have zero calls
		})

		t.Run("checks claim rules", func(t *testing.T) {
			ait := newInternalIssuerAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.other["custom-claim"] = "oops wrong claim"
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "claim \"custom-claim\" must be \"custom-claim-value\"")
			ait.ii.ExpectedCalls = []*mock.Call{ait.ii.ExpectedCalls[0]} // drop call to SigningKey() -- won't occur due to claim mismatch
		})

		t.Run("JWT times checked", func(t *testing.T) {
			ait := newInternalIssuerAITester(t,
				claimTweak(func(rc *flexibleClaims) {
					rc.ExpiresAt = jwt.NewNumericDate(time.Date(2026, time.January, 1, 12, 0, 0, 0, time.Local))
				}))
			defer ait.close()
			output := ait.bearerRequest()
			err := requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "token is expired")
		})

		t.Run("signed by incorrect JWT key", func(t *testing.T) {
			keyPath := filepath.Join(t.TempDir(), "jwt-rsa-2048-bad-key.priv")
			badSigningKey, err := jwtx.InitAsymmetricSigningKey(keyPath, "RS256")
			require.NoError(t, err)

			ait := newInternalIssuerAITester(t, jwtClientSignatureTweak(func() jwtx.SigningKey {
				return badSigningKey
			}))
			defer ait.close()

			output := ait.bearerRequest()
			err = requireOutput[*auth.AuthenticationAttemptedIncorrectCredential](t, output).Error
			require.ErrorContains(t, err, "crypto/rsa: verification error")
		})
	})
}

type AuthorizedIntegrationTester struct {
	t               *testing.T
	ai              *AuthorizedIntegration
	dbAI            *auth_model.AuthorizedIntegration
	jwtSigningKey   jwtx.SigningKey
	testServer      *httptest.Server
	resetHTTPClient func()
	tweaks          []tweak
	ii              *auth.MockInternalIssuer
}

func newAITester(t *testing.T, tweaks ...tweak) *AuthorizedIntegrationTester {
	fixedTime := time.Date(2026, time.January, 1, 16, 0, 0, 0, time.Local)
	ait := &AuthorizedIntegrationTester{
		t: t,
		ai: &AuthorizedIntegration{
			fixedTime: &fixedTime,
		},
		tweaks: tweaks,
	}
	for _, tweak := range ait.tweaks {
		if aiTweak, is := tweak.(aiTweak); is {
			aiTweak(ait.ai)
		}
	}

	var jwtSigningKey jwtx.SigningKey
	for _, tweak := range ait.tweaks {
		if jwtxKeyTweak, is := tweak.(jwtxKeyTweak); is {
			jwtSigningKey = jwtxKeyTweak()
		}
	}
	if jwtSigningKey == nil {
		var err error
		keyPath := filepath.Join(t.TempDir(), "jwt-rsa-2048.priv")
		jwtSigningKey, err = jwtx.InitAsymmetricSigningKey(keyPath, "RS256")
		require.NoError(t, err)
	}
	ait.jwtSigningKey = jwtSigningKey

	ait.testServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/actions/.well-known/openid-configuration" {
			retval := &auth.AuthorizedIntegrationOpenIDConfiguration{
				Issuer:                           ait.dbAI.Issuer,
				IDTokenSigningAlgValuesSupported: []string{"RS256"},
				JwksURI:                          fmt.Sprintf("%s/.keys", ait.dbAI.Issuer),
			}
			for _, tweak := range ait.tweaks {
				if tweak, is := tweak.(openIDTweak); is {
					tweak(retval, ait)
				}
			}
			err := json.NewEncoder(w).Encode(retval)
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/api/actions/.keys" {
			jwk, err := ait.jwtSigningKey.ToJWK()
			require.NoError(t, err)
			jwk["use"] = "sig"
			jwkMapAny := make(map[string]any, len(jwk))
			for k, v := range jwk {
				jwkMapAny[k] = v // convert map[string]string -> map[string]any
			}
			retval := &auth.AuthorizedIntegrationOpenIDKeys{
				Keys: []map[string]any{jwkMapAny},
			}
			for _, tweak := range ait.tweaks {
				if jwksTweak, is := tweak.(jwksTweak); is {
					jwksTweak(retval)
				}
			}
			_ = json.NewEncoder(w).Encode(retval) // no error checking -- some tests abort read
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	// trust TLS cert of our mock client by inserting the test client for our test server into the global aiHTTPClient
	ait.resetHTTPClient = test.MockVariableValue(
		&auth.GetAuthorizedIntegrationHTTPClient,
		func() *http.Client {
			return ait.testServer.Client()
		},
	)

	ait.dbAI = &auth_model.AuthorizedIntegration{
		UserID:   2,
		Scope:    auth_model.AccessTokenScopeAll,
		Issuer:   fmt.Sprintf("%s/api/actions", ait.testServer.URL),
		Audience: fmt.Sprintf("https://forgejo.example.org/-/coolguy/authorized-integration/%s", gouuid.New().String()),
		ClaimRules: &auth_model.ClaimRules{
			Rules: []auth_model.ClaimRule{
				{
					Claim:      "custom-claim",
					Comparison: auth_model.ClaimEqual,
					Value:      "custom-claim-value",
				},
			},
		},
	}
	for _, tweak := range ait.tweaks {
		if tweak, is := tweak.(aiDBTweak); is {
			tweak(ait.dbAI)
		}
	}
	_, err := db.GetEngine(t.Context()).Insert(ait.dbAI)
	require.NoError(t, err)

	return ait
}

func newInternalIssuerAITester(t *testing.T, tweaks ...tweak) *AuthorizedIntegrationTester {
	innerTweaks := []tweak{
		claimTweak(func(rc *flexibleClaims) {
			rc.Issuer = setting.AppURL + "/fake-jwt-issuer"
		}),
		aiDBTweak(func(ai *auth_model.AuthorizedIntegration) {
			ai.Issuer = "urn:forgejo:authorized-issuer:internal:test1"
		}),
	}
	innerTweaks = append(innerTweaks, tweaks...)
	ait := newAITester(t, innerTweaks...)
	ii := auth.NewMockInternalIssuer(t)
	auth.RegisterInternalIssuerForTesting(t, "/fake-jwt-issuer", ii)
	ii.On("IssuerPlaceholder").Return("urn:forgejo:authorized-issuer:internal:test1")
	ii.On("SigningKey").Return(ait.jwtSigningKey)
	ait.ii = ii
	return ait
}

func (ait *AuthorizedIntegrationTester) signedJWT() string {
	claims := flexibleClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   ait.dbAI.Issuer,
			Audience: jwt.ClaimStrings{ait.dbAI.Audience},
		},
		other: map[string]any{
			"custom-claim": "custom-claim-value",
		},
	}
	for _, tweak := range ait.tweaks {
		if tweak, is := tweak.(claimTweak); is {
			tweak(&claims)
		}
	}
	clientSigningKey := ait.jwtSigningKey
	for _, tweak := range ait.tweaks {
		if tweak, is := tweak.(jwtClientSignatureTweak); is {
			clientSigningKey = tweak()
		}
	}
	signedToken, err := clientSigningKey.JWT(claims)
	require.NoError(ait.t, err)
	return signedToken
}

func (ait *AuthorizedIntegrationTester) bearerRequest() auth.MethodOutput {
	signedToken := ait.signedJWT()
	req := httptest.NewRequest("GET", "https://forgejo.example.org", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", signedToken))
	return ait.ai.Verify(req, nil, nil)
}

func (ait *AuthorizedIntegrationTester) basicRequest() auth.MethodOutput {
	signedToken := ait.signedJWT()
	req := httptest.NewRequest("GET", "https://forgejo.example.org", nil)
	req.SetBasicAuth("", signedToken)
	return ait.ai.Verify(req, nil, nil)
}

func (ait *AuthorizedIntegrationTester) close() {
	ait.resetHTTPClient()
	ait.testServer.Close()
}

type tweak any

type claimTweak func(*flexibleClaims)

type aiTweak func(*AuthorizedIntegration)

type openIDTweak func(*auth.AuthorizedIntegrationOpenIDConfiguration, *AuthorizedIntegrationTester)

type jwksTweak func(*auth.AuthorizedIntegrationOpenIDKeys)

type aiDBTweak func(*auth_model.AuthorizedIntegration)

type jwtxKeyTweak func() jwtx.SigningKey

type jwtClientSignatureTweak func() jwtx.SigningKey
