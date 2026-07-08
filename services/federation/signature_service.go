// Copyright 2024, 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package federation

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"forgejo.org/models/forgefed"
	"forgejo.org/models/user"
	"forgejo.org/modules/activitypub"
	"forgejo.org/modules/log"

	ap "github.com/go-ap/activitypub"
)

func FindOrCreateActorKey(ctx context.Context, keyID string) (pubKey any, err error) {
	log.Trace("KeyID: %v", keyID)
	keyURL, err := url.Parse(keyID)
	if err != nil {
		return nil, err
	}

	// Check for existing user key
	_, federatedUser, err := user.FindFederatedUserByKeyID(ctx, keyURL.String())
	if err != nil {
		if !user.IsErrFederatedUserNotExists(err) {
			return nil, err
		}

		// Check for existing federation host key
		federationHost, err := forgefed.FindFederationHostByKeyID(ctx, keyURL.String())
		if err != nil {
			if !forgefed.IsErrFederationHostNotFound(err) {
				return nil, err
			}
		} else if federationHost.PublicKey.Valid {
			pubKey, err := x509.ParsePKIXPublicKey(federationHost.PublicKey.V)
			if err != nil {
				return nil, err
			}

			return pubKey, nil
		}
	} else if federatedUser.PublicKey.Valid {
		pubKey, err := x509.ParsePKIXPublicKey(federatedUser.PublicKey.V)
		if err != nil {
			return nil, err
		}

		return pubKey, nil
	}

	port := uint16(443)
	if keyURL.Port() != "" {
		port64, err := strconv.ParseUint(keyURL.Port(), 10, 16)
		if err != nil {
			return nil, err
		}
		port = uint16(port64)
	}

	hostURL := *keyURL
	// if the `Actor` is not an `Application`, check for a `FederationHost` record associated with the key ID
	// for user `Actor`s, we should already have a `FederationHost` record in the database, so we ensure that the user `Actor` matches the respective `FederationHost` URL
	// otherwise, we just match against the supplied key ID URL
	if federationHost, err := forgefed.FindFederationHostByFqdnAndPort(ctx, keyURL.Hostname(), port); err == nil {
		hostURL = federationHost.AsURL()
	}

	// Fetch missing key
	pubKey, pubKeyBytes, actor, err := fetchKeyFromAp(ctx, *keyURL, hostURL)
	if err != nil {
		return nil, err
	}

	switch actor.Type {
	case ap.PersonType:
		_, federatedUser, _, err := FindOrCreateFederatedUser(ctx, actor.ID.String())
		if err != nil {
			return nil, err
		}

		err = updateFederatedUserKey(ctx, federatedUser, pubKeyBytes, actor)
		if err != nil {
			return nil, err
		}
	case ap.ApplicationType:
		federationHost, err := FindOrCreateFederationHost(ctx, actor.ID.String())
		if err != nil {
			return nil, err
		}

		err = updateFederationHostKey(ctx, federationHost, pubKeyBytes, actor)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Fetched actortype (%s) is unhandled", actor.Type)
	}

	return pubKey, nil
}

func updateFederatedUserKey(ctx context.Context, federatedUser *user.FederatedUser, pubKeyBytes []byte, actor *ap.Actor) (err error) {
	if actor.Type != ap.PersonType {
		return fmt.Errorf("Fetched user type (%s) is not of user type Person", actor.Type)
	}

	if federatedUser.NormalizedOriginalURL != actor.ID.String() {
		return fmt.Errorf("federated user (%s) does not match the stored one %s", actor.ID, federatedUser.NormalizedOriginalURL)
	}

	federatedUser.KeyID = sql.NullString{
		String: actor.PublicKey.ID.String(),
		Valid:  true,
	}

	federatedUser.PublicKey = sql.Null[sql.RawBytes]{
		V:     pubKeyBytes,
		Valid: true,
	}

	return user.UpdateFederatedUser(ctx, federatedUser)
}

func updateFederationHostKey(ctx context.Context, federationHost *forgefed.FederationHost, pubKeyBytes []byte, actor *ap.Actor) (err error) {
	if actor.Type != ap.ApplicationType {
		return fmt.Errorf("Fetched user type (%s) is not of user type Application", actor.Type)
	}

	federationHost.KeyID = sql.NullString{
		String: actor.PublicKey.ID.String(),
		Valid:  true,
	}

	federationHost.PublicKey = sql.Null[sql.RawBytes]{
		V:     pubKeyBytes,
		Valid: true,
	}

	err = forgefed.UpdateFederationHost(ctx, federationHost)
	if err != nil {
		return err
	}

	return nil
}

func fetchKeyFromAp(ctx context.Context, keyURL, hostURL url.URL) (pubKey any, pubKeyBytes []byte, apPerson *ap.Actor, err error) {
	log.Trace("keyURL %v", keyURL)
	actionsUser := user.NewAPServerActor()

	clientFactory, err := activitypub.GetClientFactory(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	apClient, err := clientFactory.WithKeys(ctx, actionsUser, actionsUser.KeyID(), []*url.URL{&hostURL})
	if err != nil {
		return nil, nil, nil, err
	}

	b, err := apClient.GetBody(keyURL.String())
	if err != nil {
		return nil, nil, nil, err
	}

	actor := ap.ActorNew(ap.IRI(keyURL.String()), ap.ActorType)
	err = actor.UnmarshalJSON(b)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ActivityStreams object cannot be converted to actor: %w", err)
	}

	pubKeyFromAp := actor.PublicKey
	if pubKeyFromAp.PublicKeyPem == "" {
		return nil, nil, nil, ErrKeyNotFound{KeyID: keyURL.String()}
	}

	if pubKeyFromAp.ID.String() != keyURL.String() {
		return nil, nil, nil, fmt.Errorf("cannot find publicKey with id: %v in %v", keyURL, string(b))
	}

	pubKeyBytes, err = decodePublicKeyPem(pubKeyFromAp.PublicKeyPem)
	if err != nil {
		return nil, nil, nil, err
	}

	pubKey, err = x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	log.Trace("For %v fetched pubKey %v", keyURL, pubKey)
	return pubKey, pubKeyBytes, actor, err
}

func decodePublicKeyPem(pubKeyPem string) ([]byte, error) {
	block, _ := pem.Decode([]byte(pubKeyPem))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("could not decode publicKeyPem to PUBLIC KEY pem block type")
	}

	return block.Bytes, nil
}
