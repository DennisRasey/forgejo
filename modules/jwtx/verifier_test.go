// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package jwtx

import (
	"path/filepath"
	"testing"

	"forgejo.org/modules/generate"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSignVerifier(t *testing.T, signKey SigningKey, verifier *Verifier) {
	t.Helper()
	// test sign and verify
	claimsIn := jwt.RegisteredClaims{
		Issuer: "abc",
		ID:     "0815",
	}
	token, err := signKey.JWT(claimsIn)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	var claimsOut jwt.RegisteredClaims
	parsed, err := verifier.ParseWithClaims(token, &claimsOut)
	require.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, claimsIn, claimsOut)
	assert.Equal(t, &claimsIn, parsed.Claims)
}

func genKey(t *testing.T, algo string) SigningKey {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), algo+".priv")
	secret, _ := generate.NewJwtSecret()
	scfg := &SigningKeyCfg{
		Algorithm:      algo,
		PrivateKeyPath: &keyPath,
		SecretBytes:    &secret,
	}
	skey, err := InitSigningKey(&scfg)
	require.NoError(t, err)
	assert.NotNil(t, skey)
	assert.Nil(t, scfg)
	return skey
}

// SigningKey is also a VerificationKey
// but we explicitly want to test the real world case of signing with a
// signing key and verifying with a "real" (public) VerificationKey
func genVerificationKey(t *testing.T, skey SigningKey) VerificationKey {
	t.Helper()
	var keyPath string
	var secret []byte
	algo := skey.SigningMethod().Alg()

	if skey.IsSymmetric() {
		secret = skey.VerifyKey().([]byte)
	} else {
		keyPath = filepath.Join(t.TempDir(), algo+".pub")
		err := savePublicKey(keyPath, skey)
		require.NoError(t, err)
	}

	vcfg := &VerificationKeyCfg{
		Algorithm:     algo,
		SecretBytes:   &secret,
		PublicKeyPath: &keyPath,
	}
	vkey, err := InitVerificationKey(&vcfg)
	require.NoError(t, err)
	assert.NotNil(t, vkey)
	assert.Nil(t, vcfg)
	return vkey
}

// generate signing keys and add them all to the privKeyVerifier
// generate public keys for these and add them all to the pubKeyVerifier
// then check that all signatures verify OK
// check that signatures made with another set of keys do not verify ok
func TestVerfifier(t *testing.T) {
	privKeyVerifier := NewVerifier()
	pubKeyVerifier := NewVerifier()
	signingKeys := make(map[string]SigningKey, len(allowedAlgorithms))
	notinVerifierKeys := make(map[string]SigningKey, len(allowedAlgorithms))
	for algo := range allowedAlgorithms {
		skey := genKey(t, algo)
		signingKeys[algo] = skey

		err := privKeyVerifier.AddKey(skey)
		require.NoError(t, err)

		err = pubKeyVerifier.AddKey(genVerificationKey(t, skey))
		require.NoError(t, err)

		notinVerifierKeys[algo] = genKey(t, algo)
	}
	for algo, skey := range signingKeys {
		t.Run(algo, func(t *testing.T) {
			testSignVerifier(t, skey, privKeyVerifier)
			testSignVerifier(t, skey, pubKeyVerifier)
		})
	}
	// test signed by key not in privKeyVerifier
	chkInvalid := func(t *testing.T, symmetric bool, verifier *Verifier, token string) {
		t.Helper()
		_, err := verifier.Parse(token)
		if symmetric {
			require.ErrorContains(t, err, "token signature is invalid: signature is invalid")
		} else {
			require.ErrorContains(t, err, "token is unverifiable: error while executing keyfunc")
			require.ErrorContains(t, err, " key with kid ")
		}
	}

	for algo, skey := range notinVerifierKeys {
		t.Run("nokey "+algo, func(t *testing.T) {
			token, err := skey.JWT(jwt.RegisteredClaims{})
			require.NoError(t, err)

			chkInvalid(t, skey.IsSymmetric(), privKeyVerifier, token)
			chkInvalid(t, skey.IsSymmetric(), pubKeyVerifier, token)
		})
	}
}
