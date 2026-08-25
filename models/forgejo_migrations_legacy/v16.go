// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo_migrations_legacy

import "code.forgejo.org/xorm/xorm"

type FederatedUser struct {
	ID               int64  `xorm:"pk autoincr"`
	UserID           int64  `xorm:"NOT NULL"`
	ExternalID       string `xorm:"UNIQUE(federation_user_mapping) NOT NULL"`
	FederationHostID int64  `xorm:"UNIQUE(federation_user_mapping) NOT NULL"`
}

func CreateFederatedUserTable(x *xorm.Engine) error {
	return x.Sync(new(FederatedUser))
}
