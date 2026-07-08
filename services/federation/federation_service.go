// Copyright 2024, 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package federation

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"forgejo.org/models/forgefed"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/activitypub"
	fm "forgejo.org/modules/forgefed"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/validation"

	"github.com/google/uuid"
)

func Init() error {
	if !setting.Federation.Enabled {
		return nil
	}
	return initDeliveryQueue()
}

func FindOrCreateFederationHost(ctx context.Context, actorURI string) (*forgefed.FederationHost, error) {
	rawActorID, err := fm.NewActorID(actorURI)
	if err != nil {
		return nil, err
	}

	federationHost, err := forgefed.FindFederationHostByFqdnAndPort(ctx, rawActorID.Host, rawActorID.HostPort)
	if err != nil {
		if !forgefed.IsErrFederationHostNotFound(err) {
			return nil, err
		}

		federationHost, err = createFederationHostFromAP(ctx, rawActorID)
	}

	return federationHost, err
}

func FindOrCreateFederatedUser(ctx context.Context, actorURI string) (*user_model.User, *user_model.FederatedUser, *forgefed.FederationHost, error) {
	federationHost, personID, err := findFederationHost(ctx, actorURI)
	if err != nil {
		return nil, nil, nil, err
	}

	user, federatedUser, err := findFederatedUser(ctx, actorURI)
	if err == nil {
		log.Trace("Found local user: %v", user.Name)
		return user, federatedUser, federationHost, nil
	}

	if !user_model.IsErrFederatedUserNotExists(err) {
		return nil, nil, nil, err
	}

	// Fetch the remote user
	apUser, apFederatedUser, err := fetchUserFromAP(ctx, *personID, federationHost)
	if err != nil {
		return nil, nil, nil, err
	}

	// User is an alias, for example in newer Mastodon versions
	// - example.com/@example
	// - example.com/users/example
	// have the ID
	// - example.com/ap/users/<id>
	user, federatedUser, err = findFederatedUser(ctx, apFederatedUser.NormalizedOriginalURL)
	if err == nil {
		log.Trace("Resolved alias %s to %s", actorURI, apFederatedUser.NormalizedOriginalURL)
		return user, federatedUser, federationHost, nil
	}

	err = user_model.CreateFederatedUser(ctx, apUser, apFederatedUser)
	if err != nil {
		return nil, nil, nil, err
	}

	log.Trace("Created user %s with federatedUser %s from distant server", user.LogString(), federatedUser.LogString())
	return apUser, apFederatedUser, federationHost, nil
}

func findFederationHost(ctx context.Context, actorURI string) (*forgefed.FederationHost, *fm.PersonID, error) {
	federationHost, err := FindOrCreateFederationHost(ctx, actorURI)
	if err != nil {
		return nil, nil, err
	}

	actorID, err := fm.NewPersonID(actorURI, string(federationHost.NodeInfo.SoftwareName))
	if err != nil {
		return nil, nil, err
	}

	return federationHost, &actorID, nil
}

func findFederatedUser(ctx context.Context, actorURI string) (*user_model.User, *user_model.FederatedUser, error) {
	federationHost, _, err := findFederationHost(ctx, actorURI)
	if err != nil {
		return nil, nil, err
	}

	actorID, err := fm.NewPersonID(actorURI, string(federationHost.NodeInfo.SoftwareName))
	if err != nil {
		return nil, nil, err
	}

	localUser, federatedUser, err := user_model.FindFederatedUser(ctx, actorID.ID, federationHost.ID)
	if err != nil {
		return nil, nil, err
	}

	return localUser, federatedUser, nil
}

func createFederationHostFromAP(ctx context.Context, actorID fm.ActorID) (*forgefed.FederationHost, error) {
	actionsUser := user_model.NewAPServerActor()

	clientFactory, err := activitypub.GetClientFactory(ctx)
	if err != nil {
		return nil, err
	}

	uri, err := url.Parse(actorID.AsWellKnownNodeInfoURI())
	if err != nil {
		return nil, fmt.Errorf("invalid actor URI: %w", err)
	}

	client, err := clientFactory.WithKeys(ctx, actionsUser, actionsUser.KeyID(), []*url.URL{uri})
	if err != nil {
		return nil, err
	}

	body, err := client.GetBody(actorID.AsWellKnownNodeInfoURI())
	if err != nil {
		return nil, err
	}

	nodeInfoWellKnown, err := forgefed.NewNodeInfoWellKnown(body)
	if err != nil {
		return nil, err
	}

	body, err = client.GetBody(nodeInfoWellKnown.Href)
	if err != nil {
		return nil, err
	}

	nodeInfo, err := forgefed.NewNodeInfo(body)
	if err != nil {
		return nil, err
	}

	// TODO: we should get key material here also to have it immediately
	result, err := forgefed.NewFederationHost(actorID.Host, nodeInfo, actorID.HostPort, actorID.HostSchema)
	if err != nil {
		return nil, err
	}

	err = forgefed.CreateFederationHost(ctx, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func fetchUserFromAP(ctx context.Context, personID fm.PersonID, federationHost *forgefed.FederationHost) (*user_model.User, *user_model.FederatedUser, error) {
	actionsUser := user_model.NewAPServerActor()
	clientFactory, err := activitypub.GetClientFactory(ctx)
	if err != nil {
		return nil, nil, err
	}

	hostURL := federationHost.AsURL()
	apClient, err := clientFactory.WithKeys(ctx, actionsUser, actionsUser.KeyID(), []*url.URL{&hostURL})
	if err != nil {
		return nil, nil, err
	}

	body, err := apClient.GetBody(personID.AsURI())
	if err != nil {
		return nil, nil, err
	}

	person := fm.ForgePerson{}
	err = person.UnmarshalJSON(body)
	if err != nil {
		return nil, nil, err
	}

	if res, err := validation.IsValid(person); !res {
		return nil, nil, err
	}

	localFqdn, err := url.ParseRequestURI(setting.AppURL)
	if err != nil {
		return nil, nil, err
	}

	personIDFromActor, err := fm.NewPersonID(person.ID.GetLink().String(), string(federationHost.NodeInfo.SoftwareName))
	if err != nil {
		return nil, nil, err
	}
	email := fmt.Sprintf("f%v@%v", uuid.New().String(), localFqdn.Hostname())
	loginName := personIDFromActor.AsLoginName()
	name := fmt.Sprintf("@%v%v", person.PreferredUsername.String(), personIDFromActor.HostSuffix())
	fullName := person.Name.String()

	if len(person.Name) == 0 {
		fullName = name
	}

	inbox, err := url.ParseRequestURI(person.Inbox.GetLink().String())
	if err != nil {
		return nil, nil, err
	}

	pubKeyBytes, err := decodePublicKeyPem(person.PublicKey.PublicKeyPem)
	if err != nil {
		return nil, nil, err
	}

	newUser := user_model.User{
		LowerName:                    strings.ToLower(name),
		Name:                         name,
		FullName:                     fullName,
		Email:                        email,
		EmailNotificationsPreference: "disabled",
		ProhibitLogin:                true,
		Passwd:                       "",
		Salt:                         "",
		PasswdHashAlgo:               "",
		LoginName:                    loginName,
		Type:                         user_model.UserTypeActivityPubUser,
		IsAdmin:                      false,
	}

	federatedUser := user_model.FederatedUser{
		ExternalID:            personIDFromActor.ID,
		FederationHostID:      federationHost.ID,
		InboxPath:             inbox.Path,
		NormalizedOriginalURL: personIDFromActor.AsURI(),
		KeyID: sql.NullString{
			String: person.PublicKey.ID.String(),
			Valid:  true,
		},
		PublicKey: sql.Null[sql.RawBytes]{
			V:     pubKeyBytes,
			Valid: true,
		},
	}

	log.Trace("Fetched person's %v federatedUser from distant server: %s", person, federatedUser.LogString())
	return &newUser, &federatedUser, nil
}
