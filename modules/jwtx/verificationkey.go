// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jwtx

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"forgejo.org/modules/util"

	"github.com/golang-jwt/jwt/v5"
)

// The ...KeyCfg types are only used for handover from setting to signingkey
// see comment in setting/security.go

type VerificationKeyCfg struct {
	Algorithm     string
	SecretBytes   *[]byte
	PublicKeyPath *string
}

// VerificationKey represents a algorithm/key pair to sign JWTs
type VerificationKey interface {
	IsSymmetric() bool
	SigningMethod() jwt.SigningMethod
	VerifyKey() any
	ID() string
	// want? ToJWK() (map[string]string, error)
}

type hmacVerificationKey struct {
	signingMethod jwt.SigningMethod
	secret        []byte
}

func (key hmacVerificationKey) IsSymmetric() bool {
	return true
}

func (key hmacVerificationKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key hmacVerificationKey) VerifyKey() any {
	return key.secret
}

func (key hmacVerificationKey) ID() string { return "" }

type rsaVerificationKey struct {
	signingMethod jwt.SigningMethod
	key           *rsa.PublicKey
	id            string
}

func newRSAVerificationKey(signingMethod jwt.SigningMethod, key *rsa.PublicKey) (rsaVerificationKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key)
	if err != nil {
		return rsaVerificationKey{}, err
	}

	return rsaVerificationKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key rsaVerificationKey) IsSymmetric() bool {
	return false
}

func (key rsaVerificationKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key rsaVerificationKey) VerifyKey() any {
	return key.key
}

func (key rsaVerificationKey) ID() string {
	return key.id
}

type eddsaVerificationKey struct {
	signingMethod jwt.SigningMethod
	key           ed25519.PublicKey
	id            string
}

func newEdDSAVerificationKey(signingMethod jwt.SigningMethod, key ed25519.PublicKey) (eddsaVerificationKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key)
	if err != nil {
		return eddsaVerificationKey{}, err
	}

	return eddsaVerificationKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key eddsaVerificationKey) IsSymmetric() bool {
	return false
}

func (key eddsaVerificationKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key eddsaVerificationKey) VerifyKey() any {
	return key.key
}

func (key eddsaVerificationKey) ID() string {
	return key.id
}

type ecdsaVerificationKey struct {
	signingMethod jwt.SigningMethod
	key           *ecdsa.PublicKey
	id            string
}

func newECDSAVerificationKey(signingMethod jwt.SigningMethod, key *ecdsa.PublicKey) (ecdsaVerificationKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key)
	if err != nil {
		return ecdsaVerificationKey{}, err
	}

	return ecdsaVerificationKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key ecdsaVerificationKey) IsSymmetric() bool {
	return false
}

func (key ecdsaVerificationKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key ecdsaVerificationKey) VerifyKey() any {
	return key.key
}

func (key ecdsaVerificationKey) ID() string {
	return key.id
}

// CreateVerificationKey creates a signing key from an algorithm / key pair.
func CreateVerificationKey(algorithm string, key any) (VerificationKey, error) {
	signingMethod := GetSigningMethod(algorithm)
	if signingMethod == nil {
		return nil, ErrInvalidAlgorithmType{algorithm}
	}

	switch signingMethod.(type) {
	case *jwt.SigningMethodEd25519:
		publicKey, ok := key.(ed25519.PublicKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newEdDSAVerificationKey(signingMethod, publicKey)
	case *jwt.SigningMethodECDSA:
		publicKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newECDSAVerificationKey(signingMethod, publicKey)
	case *jwt.SigningMethodRSA:
		publicKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newRSAVerificationKey(signingMethod, publicKey)
	default:
		secret, ok := key.([]byte)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return hmacVerificationKey{signingMethod, secret}, nil
	}
}

// loadPublicKey loads a public key
func loadPublicKey(keyPath string) (any, error) {
	bytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, fmt.Errorf("no valid PEM data found in %s", keyPath)
	} else if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("expected PUBLIC KEY, got %s in %s", block.Type, keyPath)
	}

	return x509.ParsePKIXPublicKey(block.Bytes)
}

// InitVerificationKey creates a signing key from VerificationKeyCfg
// cfgP is set to nil to mark that is has been processed
func InitVerificationKey(cfgP **VerificationKeyCfg) (VerificationKey, error) {
	cfg := *cfgP
	*cfgP = nil
	var err error
	var key VerificationKey

	if IsValidSymmetricAlgorithm(cfg.Algorithm) {
		key, err = CreateVerificationKey(cfg.Algorithm, *cfg.SecretBytes)
	} else if IsValidAsymmetricAlgorithm(cfg.Algorithm) {
		key, err = InitAsymmetricVerificationKey(*cfg.PublicKeyPath, cfg.Algorithm)
	} else {
		// should never happen, setting.loadVerificationKeyCfg() ensures
		err = ErrInvalidAlgorithmType{Algorithm: cfg.Algorithm}
	}

	return key, err
}

// InitAsymmetricVerificationKey creates an asymmetric signing key from settings or creates a random key.
func InitAsymmetricVerificationKey(keyPath, algorithm string) (VerificationKey, error) {
	var err error
	var key any

	if !IsValidAsymmetricAlgorithm(algorithm) {
		return nil, ErrInvalidAlgorithmType{Algorithm: algorithm}
	}

	key, err = loadPublicKey(keyPath)
	if err == nil {
		return CreateVerificationKey(algorithm, key)
	}

	// We deliberately support loading a signing key as a verification key
	// such that users can just move a private key to another location to use it
	// as a verification key
	key, err2 := loadPrivateKey(keyPath)
	if err2 != nil {
		return nil, fmt.Errorf("Neither public key nor private key: %w/%w", err, err2)
	}

	signingKey, err2 := CreateSigningKey(algorithm, key)
	if err2 != nil {
		return nil, err2
	}
	return signingKey.(VerificationKey), nil
}
