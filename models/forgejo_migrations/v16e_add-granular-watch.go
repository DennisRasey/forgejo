// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"code.forgejo.org/xorm/xorm"
)

type WatchSource bool

const (
	// WatchSourceExplicit means the user explicitly chose to watch certain things (or none or all) of this repo.
	// It means that setting.Service.AutoWatchOnChanges doesn't have an effect on this user for this repo; they explicitly made their choice after all.
	// This mode replaces the old WatchModeDont and WatchModeNormal states.
	WatchSourceExplicit WatchSource = false
	// WatchSourceAutomatic means the user didn't explicitly select whether to watch this repo or not.
	// Instead, the user either doesn't watch the repo because they didn't ever click the watch/unwatch button.
	// Or they do watch the repo but only because the user pushed to the repo and setting.Service.AutoWatchOnChanges is true.
	// When there is no record in the db this is the same as WatchSourceAutomatic combined with all watch selections turned off (i.e., not watching anything).
	// This used to be WatchModeNone.
	// When in this mode the watch selection is never fully deselected.
	// Otherwise there'd be some automatic method to unwatch a repo; which does not exist.
	// This mode replaces the old WatchModeAuto and WatchModeNone states.
	WatchSourceAutomatic WatchSource = true

	// There may not be more modes than the above two.
	// I intend this to be a single bit.
)

func init() {
	registerMigration(&Migration{
		Description: "Add granular watch settings to repos.",
		Upgrade:     addGranularWatchColumnsAndDropModeColumn,
	})
}

func addGranularWatchColumnsAndDropModeColumn(x *xorm.Engine) error {
	type Watch struct {
		Source                     WatchSource `xorm:"BOOL DEFAULT TRUE"`
		WatchSelectionIssues       bool        `xorm:"BOOL DEFAULT TRUE"`
		WatchSelectionPullRequests bool        `xorm:"BOOL DEFAULT TRUE"`
		WatchSelectionReleases     bool        `xorm:"BOOL DEFAULT TRUE"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(Watch))
	if err != nil {
		return err
	}

	// copy of old code //
	type WatchMode uint8
	const (
		// WatchModeNone don't watch
		// This means there is no Watch record in the db.
		// We never store this mode in the db and instead remove the record from the db.
		// Furthermore, this means there is a WatchMode for all combinations of user and repo.
		// We never go back to this state once we've been in a different state.
		WatchModeNone WatchMode = iota // 0
		// WatchModeNormal watch repository (from other sources)
		// This means the user explicitly chose to watch the repo.
		WatchModeNormal // 1
		// WatchModeDont explicit don't auto-watch
		// This means the user explicitly removed themselves as a watcher.
		// Then the AutoWatchOnChanges feature doesn't make the user a watcher when they push to the repo.
		WatchModeDont // 2
		// WatchModeAuto watch repository (from AutoWatchOnChanges)
		// This is used when the user pushed to the repo and setting.Service.AutoWatchOnChanges is true.
		// That way we can differentiate people explicitly watching the repo and people only watching it because of the AutoWatchOnChanges feature.
		WatchModeAuto // 3
	)
	// end copy of old code //

	_, err = x.Exec("UPDATE `watch` SET source = ? WHERE mode IN (?, ?)", WatchSourceAutomatic, WatchModeNone, WatchModeAuto)
	if err != nil {
		return err
	}
	_, err = x.Exec("UPDATE `watch` SET source = ? WHERE mode IN (?, ?)", WatchSourceExplicit, WatchModeNormal, WatchModeDont)
	if err != nil {
		return err
	}

	_, err = x.Exec("UPDATE `watch` SET watch_selection_issues = ? WHERE mode = ? OR mode = ?", false, WatchModeNone, WatchModeDont)
	if err != nil {
		return err
	}
	_, err = x.Exec("UPDATE `watch` SET watch_selection_pull_requests = ? WHERE mode = ? OR mode = ?", false, WatchModeNone, WatchModeDont)
	if err != nil {
		return err
	}
	_, err = x.Exec("UPDATE `watch` SET watch_selection_releases = ? WHERE mode = ? OR mode = ?", false, WatchModeNone, WatchModeDont)
	if err != nil {
		return err
	}
	_, err = x.Exec("UPDATE `watch` SET watch_selection_issues = ? WHERE mode = ? OR mode = ?", true, WatchModeNormal, WatchModeAuto)
	if err != nil {
		return err
	}
	_, err = x.Exec("UPDATE `watch` SET watch_selection_pull_requests = ? WHERE mode = ? OR mode = ?", true, WatchModeNormal, WatchModeAuto)
	if err != nil {
		return err
	}
	_, err = x.Exec("UPDATE `watch` SET watch_selection_releases = ? WHERE mode = ? OR mode = ?", true, WatchModeNormal, WatchModeAuto)
	if err != nil {
		return err
	}

	_, err = x.Exec("ALTER TABLE watch DROP COLUMN `mode`")
	if err != nil {
		return err
	}
	return nil
}
