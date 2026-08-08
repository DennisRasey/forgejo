// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package jwtx

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	t.Run("RSA-2048", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-rsa-2048.priv")
		algorithm := "RS256"

		parsedKey := loadKey(t, keyPath, algorithm)

		rsaPrivateKey := parsedKey.(*rsa.PrivateKey)
		assert.Equal(t, 2048, rsaPrivateKey.N.BitLen())

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
	})

	t.Run("RSA-4096", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-rsa-4096.priv")
		algorithm := "RS512"

		parsedKey := loadKey(t, keyPath, algorithm)

		rsaPrivateKey := parsedKey.(*rsa.PrivateKey)
		assert.Equal(t, 4096, rsaPrivateKey.N.BitLen())
	})

	t.Run("ECDSA-256", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-ecdsa-256.priv")
		algorithm := "ES256"

		parsedKey := loadKey(t, keyPath, algorithm)

		ecdsaPrivateKey := parsedKey.(*ecdsa.PrivateKey)
		assert.Equal(t, 256, ecdsaPrivateKey.Params().BitSize)
	})

	t.Run("ECDSA-384", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-ecdsa-384.priv")
		algorithm := "ES384"

		parsedKey := loadKey(t, keyPath, algorithm)

		ecdsaPrivateKey := parsedKey.(*ecdsa.PrivateKey)
		assert.Equal(t, 384, ecdsaPrivateKey.Params().BitSize)
	})

	t.Run("ECDSA-512", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-ecdsa-512.priv")
		algorithm := "ES512"

		parsedKey := loadKey(t, keyPath, algorithm)

		ecdsaPrivateKey := parsedKey.(*ecdsa.PrivateKey)
		assert.Equal(t, 521, ecdsaPrivateKey.Params().BitSize)
	})

	t.Run("EdDSA", func(t *testing.T) {
		keyPath := filepath.Join(t.TempDir(), "jwt-eddsa.priv")
		algorithm := "EdDSA"

		parsedKey := loadKey(t, keyPath, algorithm)

		assert.NotNil(t, parsedKey.(ed25519.PrivateKey))
	})
}

type testClaims struct {
	Foo string `json:"Foo"`
	jwt.RegisteredClaims
}

func TestJWTHasKid(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "jwt-rsa-2048.priv")
	algorithm := "RS256"
	key, err := InitAsymmetricSigningKey(keyPath, algorithm)
	require.NoError(t, err)

	claimsIn := testClaims{
		Foo:              "bar",
		RegisteredClaims: jwt.RegisteredClaims{},
	}
	token, err := key.JWT(&claimsIn)
	require.NoError(t, err)

	var claimsOut testClaims
	parsed, err := jwt.ParseWithClaims(token, &claimsOut, func(valToken *jwt.Token) (any, error) {
		assert.NotNil(t, valToken.Method)
		assert.Equal(t, key.SigningMethod().Alg(), valToken.Method.Alg())
		kid, ok := valToken.Header["kid"]
		assert.True(t, ok)
		assert.NotNil(t, kid)

		return key.VerifyKey(), nil
	})
	require.NoError(t, err)
	assert.NotNil(t, parsed)
	assert.Equal(t, "bar", parsed.Claims.(*testClaims).Foo)
	assert.Equal(t, "bar", claimsOut.Foo)
	// dup to keyFunc above
	kid, ok := parsed.Header["kid"]
	assert.True(t, ok)
	assert.NotNil(t, kid)
}

func TestCannotCreatePrivateKey(t *testing.T) {
	_, err := InitAsymmetricSigningKey("/directory-does-not-exist-and-you-should-not-have-permission-to-create/privatekey.pem", "RS256")
	require.Error(t, err)
	require.ErrorContains(t, err, "Error generating private key")
}
