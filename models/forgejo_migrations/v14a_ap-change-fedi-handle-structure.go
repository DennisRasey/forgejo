// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

// Due to a mistake during code review, this code was merged with the prefix 14a
// but this code was merged for the v15 cycle, the correct prefix would be 15a.
// As it would lead to breakage for instance who already ran with the old prefix
// the incorrect prefix is kept.

package forgejo_migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/validation"

	"code.forgejo.org/xorm/xorm"
)

func init() {
	registerMigration(&Migration{
		Description: "use structure @PreferredUsername@host.tld:port for actors",
		Upgrade:     changeActivityPubUsernameFormat,
	})
}

func changeActivityPubUsernameFormat(x *xorm.Engine) error {
	type FederationHost struct {
		ID         int64              `xorm:"pk autoincr"`
		HostFqdn   string             `xorm:"host_fqdn UNIQUE(federation_host) INDEX VARCHAR(255) NOT NULL"`
		HostPort   uint16             `xorm:" UNIQUE(federation_host) INDEX NOT NULL DEFAULT 443"`
		HostSchema string             `xorm:"NOT NULL DEFAULT 'https'"`
		Created    timeutil.TimeStamp `xorm:"created"`
		Updated    timeutil.TimeStamp `xorm:"updated"`
	}
	type FederatedUser struct {
		ID                    int64                  `xorm:"pk autoincr"`
		UserID                int64                  `xorm:"NOT NULL INDEX user_id"`
		ExternalID            string                 `xorm:"UNIQUE(federation_user_mapping) NOT NULL"`
		FederationHostID      int64                  `xorm:"UNIQUE(federation_user_mapping) NOT NULL"`
		KeyID                 sql.NullString         `xorm:"key_id UNIQUE"`
		PublicKey             sql.Null[sql.RawBytes] `xorm:"BLOB"`
		InboxPath             string
		NormalizedOriginalURL string // This field is just to keep original information. Pls. do not use for search or as ID!
	}
	type User struct {
		ID          int64              `xorm:"pk autoincr"`
		LowerName   string             `xorm:"UNIQUE NOT NULL"`
		Name        string             `xorm:"UNIQUE NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	deleteFederatedUser := func(ctx context.Context, userID int64) error {
		_, err := db.GetEngine(ctx).Delete(&FederatedUser{UserID: userID})
		return err
	}
	userLogString := func(u *User) string {
		if u == nil {
			return "<User nil>"
		}
		return fmt.Sprintf("<User %d:%s>", u.ID, u.Name)
	}

	// Normally, the db.WithTx statement ensures that the database transaction (aka. all changes made
	// by this migration) will only be committed if the SQL operations inside of the iteration
	// (db.Iterate) don't return an error.
	//
	// This migration was originally authored with those cases in mind, but it was later agreed that
	// migrations concerning Forgejo's federation-related components should not return any errors at
	// this point in time, as federation is not considered to be stable at the moment. For more
	// information, check the relevant discussion here:
	// https://codeberg.org/forgejo-contrib/federation/issues/67
	//
	// Nevertheless, this structure involves some useful boilerplate that can be used for future
	// migrations at a later point and has been kept as-is.
	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		// The transaction is committed only if modifying all federated users is possible.
		return db.Iterate(ctx, nil, func(ctx context.Context, federatedUser *FederatedUser) error {
			// localUser represents the "local" representation of an ActivityPub (federated) user
			localUser := &User{}
			has, err := db.GetEngine(ctx).ID(federatedUser.UserID).Get(localUser)
			if err != nil {
				log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: Database error occurred while getting local user (ID: %d), ignoring...: %e", federatedUser.UserID, err)
				return nil
			}

			if !has {
				log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: User missing for federated user: %v", federatedUser)
				err := deleteFederatedUser(ctx, federatedUser.UserID)
				if err != nil {
					log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: Database error occurred while deleting federated user (%s), ignoring...: %e", federatedUser, err)
					return nil
				}
			}

			if validation.IsValidActivityPubUsername(localUser.Name) {
				log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: FederatedUser was already migrated: %v", federatedUser)
			} else {
				// Copied from models/forgefed/federationhost_repository.go (forgefed.GetFederationHost),
				// minus some validation code for FederationHost which we do not otherwise manipulate here.
				federationHost := new(FederationHost)
				has, err := db.GetEngine(ctx).ID(federatedUser.FederationHostID).Get(federationHost)
				if err != nil {
					log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: Database error occurred while looking up federation host info (for %v), ignoring...: %e", federatedUser, err)
					return nil
				} else if !has {
					log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: Federation host for federated user %s is missing", federatedUser)
					return nil
				}

				// Take part of the username before the first dash, reconstruct the rest
				// of it using whatever we have in FederationHost. Before this migration,
				// usernames of ActivityPub accounts have an expected format of
				// "username-subdomain-domain-tld-port". We don't know how many subdomains
				// there are, but that doesn't matter. We can always get the username unless
				// if the username of an ActivityPub account was manually changed by an admin,
				// in which case they should either delete the account or change it back.
				s := strings.Split(localUser.Name, "-")
				if len(s) == 0 {
					log.Warn(
						"Migration[v14a_ap-change-fedi-handle-structure]: Username %s belonging to federatedUser %v does not contain any dashes, can't construct new username",
						localUser.Name,
						federatedUser,
					)
					return nil
				}

				// Were a running Forgejo instance to create a new federated account, would the port
				// have been marked as "supplemented" (thus leading to its omission)?
				var newUsername string
				if (federationHost.HostPort == 443 && federationHost.HostSchema == "https") || (federationHost.HostPort == 80 && federationHost.HostSchema == "http") {
					newUsername = fmt.Sprintf("@%s@%s", s[0], federationHost.HostFqdn)
				} else {
					newUsername = fmt.Sprintf("@%s@%s:%d", s[0], federationHost.HostFqdn, federationHost.HostPort)
				}

				// Implicitly assumes that there won't be a lower name unique constraint violation.
				// Potentially a bit paranoid, but why not?
				userThatShouldntExist := &User{}
				lowernameTaken, err := db.GetEngine(ctx).Where("lower_name = ?", strings.ToLower(newUsername)).Table("user").Get(userThatShouldntExist)
				if err != nil {
					log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: Database error occurred, skipping migration of %s: %e", userLogString(localUser), err)
					return nil
				}

				if lowernameTaken {
					log.Warn(
						"Migration[v14a_ap-change-fedi-handle-structure]: New username %s for %s already taken by %s, deleting the former...",
						newUsername,
						userLogString(localUser),
						userLogString(userThatShouldntExist),
					)
					err := deleteFederatedUser(ctx, localUser.ID)
					if err != nil {
						log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: Database error occurred while deleting federated user (%s), ignoring...: %e", userLogString(localUser), err)
					}
					return nil
				}

				// Safe to assume that the following operations should just work now.
				log.Info("Migration[v14a_ap-change-fedi-handle-structure]: Updating username of %s to %s", userLogString(localUser), newUsername)
				if _, err := db.GetEngine(ctx).ID(localUser.ID).Cols("lower_name", "name").Update(&User{
					LowerName: strings.ToLower(newUsername),
					Name:      newUsername,
				}); err != nil {
					log.Warn("Migration[v14a_ap-change-fedi-handle-structure]: Database error occurred when updating federated user's username (%s), ignoring...: %e", userLogString(localUser), err)
					return nil
				}
			}

			return nil
		})
	})
}
