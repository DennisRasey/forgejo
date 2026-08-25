// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import "code.forgejo.org/xorm/xorm"

func AddTrustModelToRepository(x *xorm.Engine) error {
	type Repository struct {
		TrustModel int
	}
	return x.Sync(new(Repository))
}
