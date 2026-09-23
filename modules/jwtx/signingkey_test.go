// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package jwtx

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"forgejo.org/modules/generate"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSignVerify(t *testing.T, signKey SigningKey, verifyKey VerificationKey) {
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
	parsed, err := jwt.ParseWithClaims(token, &claimsOut, func(valToken *jwt.Token) (any, error) {
		assert.NotNil(t, valToken.Method)
		assert.Equal(t, signKey.SigningMethod().Alg(), valToken.Method.Alg())
		assert.Equal(t, verifyKey.SigningMethod().Alg(), valToken.Method.Alg())

		// asymmetric keys generate JWT with a kid, symmetric not
		kid, ok := valToken.Header["kid"]
		if signKey.IsSymmetric() {
			assert.False(t, ok)
		} else {
			assert.True(t, ok)
			assert.NotNil(t, kid)
		}

		return verifyKey.VerifyKey(), nil
	})
	require.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, claimsIn, claimsOut)
	assert.Equal(t, &claimsIn, parsed.Claims)
}

// creates private key
// loads it back from the file
func TestLoadOrCreateAsymmetricKey(t *testing.T) {
	loadKey := func(t *testing.T, keyPath, algorithm string) any {
		t.Helper()
		loadOrCreateAsymmetricKey(keyPath, algorithm)

		fileContent, err := os.ReadFile(keyPath)
		require.NoError(t, err)

		block, _ := pem.Decode(fileContent)
		assert.NotNil(t, block)
		assert.Equal(t, "PRIVATE KEY", block.Type)

		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		require.NoError(t, err)

		return parsedKey
	}
	useKey := func(t *testing.T, keyPath, algorithm string) {
		t.Helper()
		// duplicates loadKey() to some extent, but uses SigningKey
		assert.NotEmpty(t, keyPath)
		scfg := &SigningKeyCfg{
			Algorithm:      algorithm,
			PrivateKeyPath: &keyPath,
		}

		// load the signing key via the settings interface
		key, err := InitSigningKey(&scfg)
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.Nil(t, scfg)
		assert.NotEmpty(t, key.ID())
		assert.Nil(t, scfg)

		testSignVerify(t, key, key)

		// load the signing key file as a verification key
		assert.NotEmpty(t, keyPath)
		vcfg := &VerificationKeyCfg{
			Algorithm:     algorithm,
			PublicKeyPath: &keyPath,
		}

		// load the verification key via the settings interface
		vkey, err := InitVerificationKey(&vcfg)
		require.NoError(t, err)
		assert.NotNil(t, key)
		assert.Equal(t, key.ID(), vkey.ID())
		assert.Nil(t, scfg)

		testSignVerify(t, key, vkey)

		// save the public key, then load it and test
		pKeyPath := keyPath + ".pub"
		err = savePublicKey(pKeyPath, key)
		require.NoError(t, err)
		vcfg = &VerificationKeyCfg{
			Algorithm:     algorithm,
			PublicKeyPath: &pKeyPath,
		}
		vkey, err = InitVerificationKey(&vcfg)
		require.NoError(t, err)
		assert.NotNil(t, vkey)
		assert.Equal(t, key.ID(), vkey.ID())
		assert.Nil(t, vcfg)

		testSignVerify(t, key, vkey)
	}
	t.Run("RSA-2048", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-rsa-2048.priv")
		algorithm := "RS256"

		parsedKey := loadKey(t, keyPath, algorithm)

		rsaPrivateKey := parsedKey.(*rsa.PrivateKey)
		assert.Equal(t, 2048, rsaPrivateKey.N.BitLen())
		t.Run("Use", func(t *testing.T) {
			useKey(t, keyPath, algorithm)
		})

		t.Run("Load key with differ specified algorithm", func(t *testing.T) {
			algorithm = "EdDSA"

			parsedKey := loadKey(t, keyPath, algorithm)
			rsaPrivateKey := parsedKey.(*rsa.PrivateKey)
			assert.Equal(t, 2048, rsaPrivateKey.N.BitLen())
		})
	})

	t.Run("RSA-3072", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-rsa-3072.priv")
		algorithm := "RS384"

		parsedKey := loadKey(t, keyPath, algorithm)

		rsaPrivateKey := parsedKey.(*rsa.PrivateKey)
		assert.Equal(t, 3072, rsaPrivateKey.N.BitLen())
		t.Run("Use", func(t *testing.T) {
			useKey(t, keyPath, algorithm)
		})
	})

	t.Run("RSA-4096", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-rsa-4096.priv")
		algorithm := "RS512"

		parsedKey := loadKey(t, keyPath, algorithm)

		rsaPrivateKey := parsedKey.(*rsa.PrivateKey)
		assert.Equal(t, 4096, rsaPrivateKey.N.BitLen())
		t.Run("Use", func(t *testing.T) {
			useKey(t, keyPath, algorithm)
		})
	})

	t.Run("ECDSA-256", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-ecdsa-256.priv")
		algorithm := "ES256"

		parsedKey := loadKey(t, keyPath, algorithm)

		ecdsaPrivateKey := parsedKey.(*ecdsa.PrivateKey)
		assert.Equal(t, 256, ecdsaPrivateKey.Params().BitSize)
		t.Run("Use", func(t *testing.T) {
			useKey(t, keyPath, algorithm)
		})
	})

	t.Run("ECDSA-384", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-ecdsa-384.priv")
		algorithm := "ES384"

		parsedKey := loadKey(t, keyPath, algorithm)

		ecdsaPrivateKey := parsedKey.(*ecdsa.PrivateKey)
		assert.Equal(t, 384, ecdsaPrivateKey.Params().BitSize)
		t.Run("Use", func(t *testing.T) {
			useKey(t, keyPath, algorithm)
		})
	})

	t.Run("ECDSA-512", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-ecdsa-512.priv")
		algorithm := "ES512"

		parsedKey := loadKey(t, keyPath, algorithm)

		ecdsaPrivateKey := parsedKey.(*ecdsa.PrivateKey)
		assert.Equal(t, 521, ecdsaPrivateKey.Params().BitSize)
		t.Run("Use", func(t *testing.T) {
			useKey(t, keyPath, algorithm)
		})
	})

	t.Run("EdDSA", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-eddsa.priv")
		algorithm := "EdDSA"

		parsedKey := loadKey(t, keyPath, algorithm)

		assert.NotNil(t, parsedKey.(ed25519.PrivateKey))
		t.Run("Use", func(t *testing.T) {
			useKey(t, keyPath, algorithm)
		})
	})
}

func TestCannotCreatePrivateKey(t *testing.T) {
	_, err := InitAsymmetricSigningKey("/directory-does-not-exist-and-you-should-not-have-permission-to-create/privatekey.pem", "RS256")
	require.Error(t, err)
	require.ErrorContains(t, err, "Error generating private key")
}

// test symmetric algorithms used via the SigningKey and VerificationKey
// interfaces
func TestSymmetricKey(t *testing.T) {
	algorithms := []string{"HS256", "HS384", "HS512"}
	for _, algorithm := range algorithms {
		t.Run(algorithm, func(t *testing.T) {
			secret, _ := generate.NewJwtSecret()
			assert.NotEmpty(t, secret)

			// init the signing key via the settings interface
			scfg := &SigningKeyCfg{
				Algorithm:   algorithm,
				SecretBytes: &secret,
			}
			skey, err := InitSigningKey(&scfg)
			require.NoError(t, err)
			assert.NotNil(t, skey)
			assert.Nil(t, scfg)

			testSignVerify(t, skey, skey)

			// init the same key as a VerificationKey
			vcfg := &VerificationKeyCfg{
				Algorithm:   algorithm,
				SecretBytes: &secret,
			}
			vkey, err := InitVerificationKey(&vcfg)
			require.NoError(t, err)
			assert.NotNil(t, vkey)
			assert.Nil(t, vcfg)

			testSignVerify(t, skey, vkey)
		})
	}
}

func TestParseJWKToPublicKey(t *testing.T) {
	t.Run("EC", func(t *testing.T) {
		parsed, err := ParseJWKToPublicKey(map[string]any{
			"alg": "ES256",
			"crv": "P-256",
			"kid": "8vZnIolpXEhEPXV4YXE5q1vyATme7w3rCFOmFz27WTo",
			"kty": "EC",
			"use": "sig",
			"x":   "BL3e9VeZAjpUBUu8R91E-UHLWuSxFetd4ekIcJaC5_4",
			"y":   "4TEMrpRx_Scu0c5Y7_aKWZjwCbU3vDtMaHEia2N4vNY",
		})
		require.NoError(t, err)

		known, err := ecdsa.ParseUncompressedPublicKey(
			elliptic.P256(),
			[]byte{
				// uncompressed marker
				0x4,
				// x
				0x4, 0xbd, 0xde, 0xf5, 0x57, 0x99, 0x2, 0x3a, 0x54, 0x5, 0x4b, 0xbc, 0x47, 0xdd, 0x44, 0xf9, 0x41, 0xcb, 0x5a, 0xe4, 0xb1, 0x15, 0xeb, 0x5d, 0xe1, 0xe9, 0x8, 0x70, 0x96, 0x82, 0xe7, 0xfe,
				// y
				0xe1, 0x31, 0xc, 0xae, 0x94, 0x71, 0xfd, 0x27, 0x2e, 0xd1, 0xce, 0x58, 0xef, 0xf6, 0x8a, 0x59, 0x98, 0xf0, 0x9, 0xb5, 0x37, 0xbc, 0x3b, 0x4c, 0x68, 0x71, 0x22, 0x6b, 0x63, 0x78, 0xbc, 0xd6,
			},
		)
		require.NoError(t, err)

		require.True(t, known.Equal(parsed))
	})
}
