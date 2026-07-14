// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package jwtx

import (
	"fmt"
	"slices"

	"github.com/golang-jwt/jwt/v5"
)

// The Verifier centralizes accepted VerificationKeys and the jwtx.Parser to be
// used
//
// VerificationKeys are organized:
// - as a slice map[method] for symmetric keys (all will be tried)
// - as a map[method+kid] for asymmetric keys (only one will be tried)

type (
	byMethod    jwt.SigningMethod
	byMethodKid struct {
		method jwt.SigningMethod
		kid    string
	}
)

type Verifier struct {
	Parser          *jwt.Parser
	KeysByMethod    map[byMethod]jwt.VerificationKeySet
	KeysByMethodKid map[byMethodKid]jwt.VerificationKey
}

// NB: Do not use jwt.WithValidMethods() here, because we set the allowed
// methods in AddKey()
func NewVerifierWithParser(parser *jwt.Parser) *Verifier {
	return &Verifier{
		Parser:          parser,
		KeysByMethod:    make(map[byMethod]jwt.VerificationKeySet),
		KeysByMethodKid: make(map[byMethodKid]jwt.VerificationKey),
	}
}

// XXX TODO should add options which require exp
func NewVerifier() *Verifier {
	return NewVerifierWithParser(jwt.NewParser())
}

// collect all methods from Verifier into slice
func (verifier *Verifier) allAlgs() []string {
	algs := make([]string, 0, len(verifier.KeysByMethod)+len(verifier.KeysByMethodKid))
	for method := range verifier.KeysByMethod {
		algs = append(algs, method.Alg())
	}
	for mk := range verifier.KeysByMethodKid {
		algs = append(algs, mk.method.Alg())
	}
	return algs
}

// Adding is idempotent (adding multiple times is not an error)
// Trying to add different keys with the same method+kid is an error
func (verifier *Verifier) AddKey(vkey VerificationKey) error {
	kid := vkey.ID()
	key, ok := vkey.VerifyKey().(jwt.VerificationKey)
	if !ok {
		return fmt.Errorf("Can not cast %s VerificationKey to jwt.VerificationKey", vkey.SigningMethod().Alg())
	}

	method := vkey.SigningMethod()
	algorithm := vkey.SigningMethod().Alg()

	err := func() error {
		if kid == "" {
			keySet, found := verifier.KeysByMethod[method]
			if !found {
				verifier.KeysByMethod[method] = jwt.VerificationKeySet{Keys: []jwt.VerificationKey{key}}
				return nil
			}

			if slices.Contains(keySet.Keys, key) {
				return nil
			}
			keySet.Keys = append(keySet.Keys, key)
			return nil
		}
		idx := byMethodKid{method, kid}
		existing, found := verifier.KeysByMethodKid[idx]
		if found && existing != key {
			return fmt.Errorf("Can not add different %s keys with key ID %s", algorithm, kid)
		}
		if !found {
			verifier.KeysByMethodKid[idx] = key
		}
		return nil
	}()
	if err != nil {
		// This is inefficient, because we reset the list of parser-supported
		// algorithms each time we add a key, but the intialization order via
		// jwt.NewParser() is the wrong way around for us. But then this setup is
		// only done onec, so it should not really matter (flw)
		jwt.WithValidMethods(verifier.allAlgs())(verifier.Parser)
	}
	return err
}

func (verifier *Verifier) AddVerificationKeyCfg(cfgP *[]*VerificationKeyCfg) error {
	cfg := *cfgP
	*cfgP = nil

	if cfg == nil {
		return nil
	}

	for _, vcfg := range cfg {
		vkey, err := InitVerificationKey(&vcfg)
		if err != nil {
			return err
		}
		err = verifier.AddKey(vkey)
		if err != nil {
			return err
		}
	}
	return nil
}

func (verifier *Verifier) ParseWithClaims(tokenString string, claims jwt.Claims) (*jwt.Token, error) {
	return verifier.Parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		kid, hasKid := token.Header["kid"]
		if hasKid {
			key, found := verifier.KeysByMethodKid[byMethodKid{token.Method, kid.(string)}]
			if found {
				return key, nil
			}
			return nil, fmt.Errorf("%s key with kid \"%s\" not found", token.Method.Alg(), kid)
		}
		keys, found := verifier.KeysByMethod[token.Method]
		if found {
			return keys, nil
		}
		return nil, fmt.Errorf("No %s keys found", token.Method.Alg())
	})
}

func (verifier *Verifier) Parse(tokenString string) (*jwt.Token, error) {
	return verifier.ParseWithClaims(tokenString, jwt.MapClaims{})
}
