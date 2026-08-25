// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.forgejo.org/xorm/xorm"
)

func AddWikiBranchToRepository(x *xorm.Engine) error {
	type Repository struct {
		ID         int64 `xorm:"pk autoincr"`
		WikiBranch string
	}

	if err := x.Sync(&Repository{}); err != nil {
		return err
	}

	// Update existing repositories to use `master` as the wiki branch, for
	// compatilibty's sake.
	_, err := x.Exec("UPDATE repository SET wiki_branch = 'master' WHERE wiki_branch = '' OR wiki_branch IS NULL")
	return err
}
