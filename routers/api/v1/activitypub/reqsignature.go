// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"

	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
	"github.com/go-fed/httpsig"
)

func getPublicKeyFromResponse(b []byte, keyID string) (p crypto.PublicKey, err error) {
	person := ap.PersonNew(ap.IRI(keyID))
	err = person.UnmarshalJSON(b)
	if err != nil {
		return nil, fmt.Errorf("ActivityStreams type cannot be converted to one known to have publicKey property: %w", err)
	}
	pubKey := person.PublicKey
	if pubKey.ID.String() != keyID {
		return nil, fmt.Errorf("cannot find publicKey with id: %s in %s", keyID, string(b))
	}
	pubKeyPem := pubKey.PublicKeyPem
	block, _ := pem.Decode([]byte(pubKeyPem))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("could not decode publicKeyPem to PUBLIC KEY pem block type")
	}
	p, err = x509.ParsePKIXPublicKey(block.Bytes)
	return p, err
}

func getKeyID(r *http.Request) (httpsig.Verifier, string, error) {
	v, err := httpsig.NewVerifier(r)
	if err != nil {
		return nil, "", err
	}
	return v, v.KeyId(), nil
}

func verifyHTTPSignatures(ctx *gitea_context.APIContext) (authenticated bool, err error) {
	r := ctx.Req

	// 1. Figure out what key we need to verify
	v, err := httpsig.NewVerifier(r)
	if err != nil {
		return false, err
	}
	ID := v.KeyId()
	// 2. Fetch the public key of the other actor
	b, err := activitypub.Fetch(ID)
	if err != nil {
		return false, err
	}
	pubKey, err := getPublicKeyFromResponse(b, ID)
	if err != nil {
		return false, err
	}
	// 3. Verify the other actor's key
	algo := httpsig.Algorithm(setting.Federation.Algorithms[0])
	authenticated = v.Verify(pubKey, algo) == nil
	if !authenticated {
		return
	}
	// 4. Create a federated user for the actor
	var person ap.Person
	err = person.UnmarshalJSON(b)
	if err != nil {
		return
	}

	err = createPerson(ctx, &person)
	return authenticated, err
}

// ReqHTTPSignature function
func ReqHTTPSignature() func(ctx *gitea_context.APIContext) {
	return func(ctx *gitea_context.APIContext) {
		if authenticated, err := verifyHTTPSignatures(ctx); err != nil {
			ctx.ServerError("verifyHttpSignatures", err)
		} else if !authenticated {
			ctx.Error(http.StatusForbidden, "reqSignature", "request signature verification failed")
		}
	}
}

// Check if the keyID matches the activity to prevent impersonation
func checkActivityAndKeyID(activity ap.Activity, keyID string) error {
	if activity.Actor != nil && keyID != activity.Actor.GetLink().String()+"#main-key" {
		return errors.New("actor does not match HTTP signature keyID")
	}
	if activity.AttributedTo != nil && keyID != activity.AttributedTo.GetLink().String()+"#main-key" {
		return errors.New("attributedTo does not match HTTP signature keyID")
	}
	if activity.Object == nil {
		return errors.New("activity does not contain object")
	}
	if activity.Type == ap.UndoType {
		return ap.OnActivity(activity.Object, func(a *ap.Activity) error {
			if a.Actor != nil && keyID != a.Actor.GetLink().String()+"#main-key" {
				// TODO: This doesn't necessarily mean impersonation since the object might be created by someone else
				return errors.New("actor does not match HTTP signature keyID")
			}
			if a.AttributedTo != nil && keyID != a.AttributedTo.GetLink().String()+"#main-key" {
				return errors.New("attributedTo does not match HTTP signature keyID")
			}
			return nil
		})
	}
	return nil
}
