// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jwtx

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"forgejo.org/modules/log"
	"forgejo.org/modules/util"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidAlgorithmType represents an invalid algorithm error.
type ErrInvalidAlgorithmType struct {
	Algorithm string
}

func (err ErrInvalidAlgorithmType) Error() string {
	return fmt.Sprintf("JWT signing algorithm is not supported: %s", err.Algorithm)
}

func jwtHelper(key SigningKey, claims jwt.Claims, opts ...jwt.TokenOption) (string, error) {
	jwt := jwt.NewWithClaims(key.SigningMethod(), claims, opts...)
	key.PreProcessToken(jwt)
	return jwt.SignedString(key.SignKey())
}

// SigningKey represents a algorithm/key pair to sign JWTs
type SigningKey interface {
	IsSymmetric() bool
	SigningMethod() jwt.SigningMethod
	SignKey() any
	VerifyKey() any
	ToJWK() (map[string]string, error)
	PreProcessToken(*jwt.Token)
	// convenience: jwt.NewWithClaims + PreProcessToken + SignedString
	JWT(jwt.Claims, ...jwt.TokenOption) (string, error)
}

type hmacSigningKey struct {
	signingMethod jwt.SigningMethod
	secret        []byte
}

func (key hmacSigningKey) IsSymmetric() bool {
	return true
}

func (key hmacSigningKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key hmacSigningKey) SignKey() any {
	return key.secret
}

func (key hmacSigningKey) VerifyKey() any {
	return key.secret
}

func (key hmacSigningKey) ToJWK() (map[string]string, error) {
	return map[string]string{
		"kty": "oct",
		"alg": key.SigningMethod().Alg(),
	}, nil
}

func (key hmacSigningKey) PreProcessToken(*jwt.Token) {}

func (key hmacSigningKey) JWT(claims jwt.Claims, opts ...jwt.TokenOption) (string, error) {
	return jwtHelper(key, claims, opts...)
}

type rsaSigningKey struct {
	signingMethod jwt.SigningMethod
	key           *rsa.PrivateKey
	id            string
}

func newRSASigningKey(signingMethod jwt.SigningMethod, key *rsa.PrivateKey) (rsaSigningKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key.Public().(*rsa.PublicKey))
	if err != nil {
		return rsaSigningKey{}, err
	}

	return rsaSigningKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key rsaSigningKey) IsSymmetric() bool {
	return false
}

func (key rsaSigningKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key rsaSigningKey) SignKey() any {
	return key.key
}

func (key rsaSigningKey) VerifyKey() any {
	return key.key.Public()
}

func (key rsaSigningKey) ToJWK() (map[string]string, error) {
	pubKey := key.key.Public().(*rsa.PublicKey)

	return map[string]string{
		"kty": "RSA",
		"alg": key.SigningMethod().Alg(),
		"kid": key.id,
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
		"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
	}, nil
}

func (key rsaSigningKey) PreProcessToken(token *jwt.Token) {
	token.Header["kid"] = key.id
}

func (key rsaSigningKey) JWT(claims jwt.Claims, opts ...jwt.TokenOption) (string, error) {
	return jwtHelper(key, claims, opts...)
}

type eddsaSigningKey struct {
	signingMethod jwt.SigningMethod
	key           ed25519.PrivateKey
	id            string
}

func newEdDSASigningKey(signingMethod jwt.SigningMethod, key ed25519.PrivateKey) (eddsaSigningKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key.Public().(ed25519.PublicKey))
	if err != nil {
		return eddsaSigningKey{}, err
	}

	return eddsaSigningKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key eddsaSigningKey) IsSymmetric() bool {
	return false
}

func (key eddsaSigningKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key eddsaSigningKey) SignKey() any {
	return key.key
}

func (key eddsaSigningKey) VerifyKey() any {
	return key.key.Public()
}

func (key eddsaSigningKey) ToJWK() (map[string]string, error) {
	pubKey := key.key.Public().(ed25519.PublicKey)

	return map[string]string{
		"alg": key.SigningMethod().Alg(),
		"kid": key.id,
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pubKey),
	}, nil
}

func (key eddsaSigningKey) PreProcessToken(token *jwt.Token) {
	token.Header["kid"] = key.id
}

func (key eddsaSigningKey) JWT(claims jwt.Claims, opts ...jwt.TokenOption) (string, error) {
	return jwtHelper(key, claims, opts...)
}

type ecdsaSigningKey struct {
	signingMethod jwt.SigningMethod
	key           *ecdsa.PrivateKey
	id            string
}

func newECDSASigningKey(signingMethod jwt.SigningMethod, key *ecdsa.PrivateKey) (ecdsaSigningKey, error) {
	kid, err := util.CreatePublicKeyFingerprint(key.Public().(*ecdsa.PublicKey))
	if err != nil {
		return ecdsaSigningKey{}, err
	}

	return ecdsaSigningKey{
		signingMethod,
		key,
		base64.RawURLEncoding.EncodeToString(kid),
	}, nil
}

func (key ecdsaSigningKey) IsSymmetric() bool {
	return false
}

func (key ecdsaSigningKey) SigningMethod() jwt.SigningMethod {
	return key.signingMethod
}

func (key ecdsaSigningKey) SignKey() any {
	return key.key
}

func (key ecdsaSigningKey) VerifyKey() any {
	return key.key.Public()
}

func (key ecdsaSigningKey) ToJWK() (map[string]string, error) {
	pubKey := key.key.Public().(*ecdsa.PublicKey)

	return map[string]string{
		"kty": "EC",
		"alg": key.SigningMethod().Alg(),
		"kid": key.id,
		"crv": pubKey.Params().Name,
		"x":   base64.RawURLEncoding.EncodeToString(pubKey.X.Bytes()), //nolint:staticcheck // no easy replacement. JWTX specification mandates marshalling to x, even if unsafe.
		"y":   base64.RawURLEncoding.EncodeToString(pubKey.Y.Bytes()), //nolint:staticcheck // no easy replacement. JWTX specification mandates marshalling to y, even if unsafe.
	}, nil
}

func (key ecdsaSigningKey) PreProcessToken(token *jwt.Token) {
	token.Header["kid"] = key.id
}

func (key ecdsaSigningKey) JWT(claims jwt.Claims, opts ...jwt.TokenOption) (string, error) {
	return jwtHelper(key, claims, opts...)
}

var allowedAlgorithms = map[string]bool{
	"HS256": true,
	"HS384": true,
	"HS512": true,

	"RS256": true,
	"RS384": true,
	"RS512": true,

	"ES256": true,
	"ES384": true,
	"ES512": true,
	"EdDSA": true,
}

func GetSigningMethod(algorithm string) jwt.SigningMethod {
	if !allowedAlgorithms[algorithm] {
		return nil
	}
	return jwt.GetSigningMethod(algorithm)
}

// CreateSigningKey creates a signing key from an algorithm / key pair.
func CreateSigningKey(algorithm string, key any) (SigningKey, error) {
	signingMethod := GetSigningMethod(algorithm)
	if signingMethod == nil {
		return nil, ErrInvalidAlgorithmType{algorithm}
	}

	switch signingMethod.(type) {
	case *jwt.SigningMethodEd25519:
		privateKey, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newEdDSASigningKey(signingMethod, privateKey)
	case *jwt.SigningMethodECDSA:
		privateKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newECDSASigningKey(signingMethod, privateKey)
	case *jwt.SigningMethodRSA:
		privateKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return newRSASigningKey(signingMethod, privateKey)
	default:
		secret, ok := key.([]byte)
		if !ok {
			return nil, jwt.ErrInvalidKeyType
		}
		return hmacSigningKey{signingMethod, secret}, nil
	}
}

func createAsymmetricKey(keyPath, algorithm string) error {
	key, err := func() (any, error) {
		switch {
		case strings.HasPrefix(algorithm, "RS"):
			var bits int
			switch algorithm {
			case "RS256":
				bits = 2048
			case "RS384":
				bits = 3072
			case "RS512":
				bits = 4096
			}
			return rsa.GenerateKey(rand.Reader, bits)
		case algorithm == "EdDSA":
			_, pk, err := ed25519.GenerateKey(rand.Reader)
			return pk, err
		default:
			var curve elliptic.Curve
			switch algorithm {
			case "ES256":
				curve = elliptic.P256()
			case "ES384":
				curve = elliptic.P384()
			case "ES512":
				curve = elliptic.P521()
			}
			return ecdsa.GenerateKey(curve, rand.Reader)
		}
	}()
	if err != nil {
		return err
	}

	bytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}

	privateKeyPEM := &pem.Block{Type: "PRIVATE KEY", Bytes: bytes}

	if err := os.MkdirAll(filepath.Dir(keyPath), os.ModePerm); err != nil {
		return err
	}

	f, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err = f.Close(); err != nil {
			log.Error("Close: %v", err)
		}
	}()

	return pem.Encode(f, privateKeyPEM)
}

func loadAsymmetricKey(keyPath string) (any, error) {
	bytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, fmt.Errorf("no valid PEM data found in %s", keyPath)
	} else if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("expected PRIVATE KEY, got %s in %s", block.Type, keyPath)
	}

	return x509.ParsePKCS8PrivateKey(block.Bytes)
}

// loadOrCreateAsymmetricKey checks if the configured private key exists.
// If it does not exist a new random key gets generated and saved on the configured path.
func loadOrCreateAsymmetricKey(keyPath, algorithm string) (any, error) {
	isExist, err := util.IsExist(keyPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to check if %s exists. Error: %v", keyPath, err)
	}
	if !isExist {
		err := createAsymmetricKey(keyPath, algorithm)
		if err != nil {
			return nil, fmt.Errorf("Error generating private key %s: %v", keyPath, err)
		}
	}
	return loadAsymmetricKey(keyPath)
}

// InitSigningKey creates a signing key from settings or creates a random key.
func InitSigningKey(getGeneralTokenSigningSecret func() []byte, keyPath, algorithm string) (SigningKey, error) {
	var err error
	var key SigningKey

	key, err = InitSymmetricSigningKey(getGeneralTokenSigningSecret, algorithm)
	if err != nil {
		key, err = InitAsymmetricSigningKey(keyPath, algorithm)
		if err != nil {
			return nil, err
		}
	}

	return key, nil
}

// IsValidSymmetricAlgorithm checks if the passed in algorithm is a supported symettric algorithm.
func IsValidSymmetricAlgorithm(algorithm string) bool {
	validAlgs := []string{"HS256", "HS384", "HS512"}

	return slices.Contains(validAlgs, algorithm)
}

// InitSymmetricSigningKey creates a symmetric signing key from settings.
func InitSymmetricSigningKey(getGeneralTokenSigningSecret func() []byte, algorithm string) (SigningKey, error) {
	var err error

	if !IsValidSymmetricAlgorithm(algorithm) {
		return nil, fmt.Errorf("invalid algorithm: %s", algorithm)
	}

	signingKey, err := CreateSigningKey(algorithm, getGeneralTokenSigningSecret())
	if err != nil {
		return nil, err
	}

	return signingKey, nil
}

// IsValidAsymmetricAlgorithm checks if the passed in algorithm is a supported asymmetric algorithm.
func IsValidAsymmetricAlgorithm(algorithm string) bool {
	validAlgs := []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "EdDSA"}

	return slices.Contains(validAlgs, algorithm)
}

// InitAsymmetricSigningKey creates an asymmetric signing key from settings or creates a random key.
func InitAsymmetricSigningKey(keyPath, algorithm string) (SigningKey, error) {
	var err error
	var key any

	if !IsValidAsymmetricAlgorithm(algorithm) {
		return nil, ErrInvalidAlgorithmType{Algorithm: algorithm}
	}

	key, err = loadOrCreateAsymmetricKey(keyPath, algorithm)
	if err != nil {
		return nil, fmt.Errorf("Error while loading or creating JWT key: %w", err)
	}

	signingKey, err := CreateSigningKey(algorithm, key)
	if err != nil {
		return nil, err
	}

	return signingKey, nil
}
