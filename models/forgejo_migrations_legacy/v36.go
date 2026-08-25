// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import (
	"code.forgejo.org/xorm/xorm"
)

func FixWikiUnitDefaultPermission(x *xorm.Engine) error {
	// Type is Unit's Type
	type Type int

	// Enumerate all the unit types
	const (
		TypeInvalid         Type = iota // 0 invalid
		TypeCode                        // 1 code
		TypeIssues                      // 2 issues
		TypePullRequests                // 3 PRs
		TypeReleases                    // 4 Releases
		TypeWiki                        // 5 Wiki
		TypeExternalWiki                // 6 ExternalWiki
		TypeExternalTracker             // 7 ExternalTracker
		TypeProjects                    // 8 Projects
		TypePackages                    // 9 Packages
		TypeActions                     // 10 Actions
	)

	// RepoUnitAccessMode specifies the users access mode to a repo unit
	type UnitAccessMode int

	const (
		// UnitAccessModeUnset - no unit mode set
		UnitAccessModeUnset UnitAccessMode = iota // 0
		// UnitAccessModeNone no access
		UnitAccessModeNone // 1
		// UnitAccessModeRead read access
		UnitAccessModeRead // 2
		// UnitAccessModeWrite write access
		UnitAccessModeWrite // 3
	)
	_ = UnitAccessModeNone
	_ = UnitAccessModeWrite

	type RepoUnit struct {
		DefaultPermissions UnitAccessMode `xorm:"NOT NULL DEFAULT 0"`
	}
	_, err := x.Where("type = ?", TypeWiki).
		Where("default_permissions = ?", UnitAccessModeRead).
		Cols("default_permissions").
		Update(RepoUnit{
			DefaultPermissions: UnitAccessModeUnset,
		})
	return err
}
