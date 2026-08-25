// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

func init() {
	registerMigration(&Migration{
		Description: "Set ProhibitLogin and UserTypeActivityPubUser for remote users created from ActivityPub.",
		Upgrade:     setProhibitLoginActivityPubUser,
	})
}

func setProhibitLoginActivityPubUser(x *xorm.Engine) error {
	type UserType int
	const (
		UserTypeIndividual           UserType = iota // Historic reason to make it starts at 0.
		UserTypeOrganization                         // 1
		UserTypeUserReserved                         // 2
		UserTypeOrganizationReserved                 // 3
		UserTypeBot                                  // 4
		UserTypeRemoteUser                           // 5
		UserTypeActivityPubUser                      // 6
	)
	type User struct {
		ID             int64  `xorm:"pk autoincr"`
		Name           string `xorm:"UNIQUE NOT NULL"`
		Passwd         string `xorm:"NOT NULL"`
		PasswdHashAlgo string `xorm:"NOT NULL DEFAULT 'argon2'"`
		Type           UserType
		Salt           string             `xorm:"VARCHAR(32)"`
		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
		ProhibitLogin  bool               `xorm:"NOT NULL DEFAULT false"`
	}
	type FederatedUser struct {
		UserID int64 `xorm:"NOT NULL INDEX user_id"`
	}

	userLogString := func(u *User) string {
		if u == nil {
			return "<User nil>"
		}
		return fmt.Sprintf("<User %d:%s>", u.ID, u.Name)
	}

	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		return db.Iterate(ctx, builder.Eq{"type": 5}, func(ctx context.Context, user *User) error {
			log.Info("Checking if user %s is created from ActivityPub", userLogString(user))

			// Users created from f3 also have the RemoteUser user type. All
			// FederatedUser should reference exactly one User.
			has, err := db.GetEngine(ctx).Table("federated_user").Get(&FederatedUser{UserID: user.ID})
			if err != nil {
				return err
			}

			if !has {
				return nil
			}

			log.Info("Updating user %s", userLogString(user))
			_, err = db.GetEngine(ctx).Table("user").ID(user.ID).Cols("type", "prohibit_login", "passwd", "salt", "passwd_hash_algo").Update(&User{
				Type:           UserTypeActivityPubUser,
				ProhibitLogin:  true,
				Passwd:         "",
				Salt:           "",
				PasswdHashAlgo: "",
			})

			return err
		})
	})
}
