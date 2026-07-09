// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"

	"forgejo.org/models/db"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"

	"xorm.io/builder"
)

// The main purpose of the WatchSource is to respect explicit user choice and not overwrite that with some automatic system.
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
)

type WatchSelection struct {
	Issues       bool
	PullRequests bool
	Releases     bool
	// When changing these options or adding more don't forget to update
	// BuilderWatchAnything and checkForRepoConsistency in the consistency check.
}

// When the user is watching at least one thing on a repo they count as a watcher on the repo.
func (w WatchSelection) IsWatching() bool {
	return w.Issues || w.PullRequests || w.Releases
}

// Return true iff the user is watching everything.
func (w WatchSelection) IsFullyWatching() bool {
	return w.Issues && w.PullRequests && w.Releases
}

var (
	WatchAllSelection  = WatchSelection{Issues: true, PullRequests: true, Releases: true}
	WatchNoneSelection = WatchSelection{Issues: false, PullRequests: false, Releases: false}
)

// Watch is connection request for receiving repository notification.
type Watch struct {
	ID     int64       `xorm:"pk autoincr"`
	UserID int64       `xorm:"UNIQUE(watch)"`
	RepoID int64       `xorm:"UNIQUE(watch)"`
	Source WatchSource `xorm:"BOOL DEFAULT TRUE"`
	// In the next PR there will be another mode here, choosing the user preset or a custom selection.
	// TODO: figure out whether the user preset should count as watching
	// TODO: (then change the description to IsWatching)

	WatchSelectionIssues       bool `xorm:"BOOL DEFAULT TRUE"`
	WatchSelectionPullRequests bool `xorm:"BOOL DEFAULT TRUE"`
	WatchSelectionReleases     bool `xorm:"BOOL DEFAULT TRUE"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func (w Watch) GetWatchSelection() WatchSelection {
	return WatchSelection{Issues: w.WatchSelectionIssues, PullRequests: w.WatchSelectionPullRequests, Releases: w.WatchSelectionReleases}
}

// Warning: this does not set the WatchSource.
// The caller needs to do that properly.
func (w *Watch) setWatchSelection(watchSelection WatchSelection) {
	w.WatchSelectionIssues = watchSelection.Issues
	w.WatchSelectionPullRequests = watchSelection.PullRequests
	w.WatchSelectionReleases = watchSelection.Releases
}

func init() {
	db.RegisterModel(new(Watch))
}

// GetWatch gets what kind of subscription a user has on a given repository; returns dummy record if none found
func GetWatch(ctx context.Context, userID, repoID int64) (Watch, error) {
	watch := Watch{UserID: userID, RepoID: repoID}
	has, err := db.GetEngine(ctx).Get(&watch)
	if err != nil {
		return watch, err
	}
	if !has {
		watch.Source = WatchSourceAutomatic
		watch.setWatchSelection(WatchNoneSelection)
	}
	return watch, nil
}

// IsWatcher checks if the user is a watcher of the repo.
func IsWatcher(ctx context.Context, userID, repoID int64) bool {
	watch, err := GetWatch(ctx, userID, repoID)
	return err == nil && watch.GetWatchSelection().IsWatching()
}

// GetWatchSelection returns what parts of a repo a user is watching.
// When you don't care if the user watches something explicitly or automatically this should suffice.
func GetWatchSelection(ctx context.Context, userID, repoID int64) WatchSelection {
	watch, err := GetWatch(ctx, userID, repoID)
	if err != nil {
		return WatchNoneSelection
	}
	return watch.GetWatchSelection()
}

// Change the watch status on the provided Watch instance.
// oldWatch contains the prior state.
func updateWatchRepo(ctx context.Context, oldWatch Watch, source WatchSource, watchSelection WatchSelection) (err error) {
	if oldWatch.Source == source && oldWatch.GetWatchSelection() == watchSelection {
		return nil
	}
	if source == WatchSourceAutomatic && oldWatch.Source == WatchSourceExplicit {
		// This should never happen and indicates a bug.
		return errors.New("we should never switch from an explicit watch state to an automatic one")
	}

	hadrec, err := db.GetEngine(ctx).Get(&oldWatch)
	if err != nil {
		return err
	}

	repodiff := 0
	if watchSelection.IsWatching() && !oldWatch.GetWatchSelection().IsWatching() {
		repodiff = 1
	} else if !watchSelection.IsWatching() && oldWatch.GetWatchSelection().IsWatching() {
		repodiff = -1
	}

	if !hadrec {
		oldWatch.Source = source
		oldWatch.setWatchSelection(watchSelection)
		if err = db.Insert(ctx, oldWatch); err != nil {
			return err
		}
	} else {
		oldWatch.Source = source
		oldWatch.setWatchSelection(watchSelection)
		if _, err := db.GetEngine(ctx).ID(oldWatch.ID).AllCols().Update(oldWatch); err != nil {
			return err
		}
	}
	// Notice that we never delete a record.
	// We could do that in the case of an automatic source and none selection.
	// But that would only save some db space and we never go into that state when we've been in a different one at some point.

	if repodiff != 0 {
		_, err = db.GetEngine(ctx).Exec("UPDATE `repository` SET num_watches = num_watches + ? WHERE id = ?", repodiff, oldWatch.RepoID)
	}
	return err
}

// WatchRepoExplicitly explicitly watch or unwatch repository according to some selection.
func WatchRepoExplicitly(ctx context.Context, userID, repoID int64, watchSelection WatchSelection) (err error) {
	var watch Watch
	if watch, err = GetWatch(ctx, userID, repoID); err != nil {
		return err
	}
	err = updateWatchRepo(ctx, watch, WatchSourceExplicit, watchSelection)
	return err
}

// GetSelectWatchers returns all users watching any of the selected parts of the repo with the given repoID.
func GetSelectWatchers(ctx context.Context, repoID int64, selection WatchSelection) ([]*Watch, error) {
	sqlSelection := []builder.Cond{}
	if selection.Issues {
		sqlSelection = append(sqlSelection, builder.Eq{"`watch`.watch_selection_issues": true})
	}
	if selection.PullRequests {
		sqlSelection = append(sqlSelection, builder.Eq{"`watch`.watch_selection_pull_requests": true})
	}
	if selection.Releases {
		sqlSelection = append(sqlSelection, builder.Eq{"`watch`.watch_selection_releases": true})
	}
	// When we don't select anything, don't return anything.
	if len(sqlSelection) == 0 {
		return []*Watch{}, nil
	}

	watches := make([]*Watch, 0, 10)
	return watches, db.GetEngine(ctx).Where("`watch`.repo_id=?", repoID).
		And(builder.Or(sqlSelection...)).
		And("`user`.is_active=?", true).
		And("`user`.prohibit_login=?", false).
		Join("INNER", "`user`", "`user`.id = `watch`.user_id").
		Find(&watches)
}

// GetSelectWatcherIDs returns IDs of users watching any of the selected parts of the repo with the given repoID.
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetSelectWatcherIDs(ctx context.Context, repoID int64, selection WatchSelection) ([]int64, error) {
	sqlSelection := []builder.Cond{}
	if selection.Issues {
		sqlSelection = append(sqlSelection, builder.Eq{"`watch`.watch_selection_issues": true})
	}
	if selection.PullRequests {
		sqlSelection = append(sqlSelection, builder.Eq{"`watch`.watch_selection_pull_requests": true})
	}
	if selection.Releases {
		sqlSelection = append(sqlSelection, builder.Eq{"`watch`.watch_selection_releases": true})
	}
	// When we don't select anything, don't return anything.
	if len(sqlSelection) == 0 {
		return []int64{}, nil
	}

	ids := make([]int64, 0, 64)
	return ids, db.GetEngine(ctx).Table("watch").
		Where("watch.repo_id=?", repoID).
		And(builder.Or(sqlSelection...)).
		Select("user_id").
		Find(&ids)
}

func BuilderWatchAnything() builder.Cond {
	return builder.Or(
		builder.Eq{"`watch`.watch_selection_issues": true},
		builder.Eq{"`watch`.watch_selection_pull_requests": true},
		builder.Eq{"`watch`.watch_selection_releases": true},
	)
}

// GetRepoWatchers returns range of users watching given repository.
func GetRepoWatchers(ctx context.Context, repoID int64, opts db.ListOptions) ([]*user_model.User, error) {
	sess := db.GetEngine(ctx).Where("watch.repo_id=?", repoID).
		Join("LEFT", "watch", "`user`.id=`watch`.user_id").
		And(BuilderWatchAnything())
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, &opts)
		users := make([]*user_model.User, 0, opts.PageSize)

		return users, sess.Find(&users)
	}

	users := make([]*user_model.User, 0, 8)
	return users, sess.Find(&users)
}

// Respect explicit user wish and don't change explicit watches.
func autoWatch(ctx context.Context, userID, repoID int64) error {
	watch, err := GetWatch(ctx, userID, repoID)
	if err != nil {
		return err
	}
	if watch.Source == WatchSourceExplicit {
		return nil
	}
	// TODO: This would be a place to check that we are using the user preset (which doesn't exist yet).
	if watch.GetWatchSelection().IsFullyWatching() {
		return nil
	}
	return updateWatchRepo(ctx, watch, WatchSourceAutomatic, WatchAllSelection)
}

// WatchIfAutoWatchOnChanges subscribes to repo if AutoWatchOnChanges is set
func WatchIfAutoWatchOnChanges(ctx context.Context, userID, repoID int64) error {
	if !setting.Service.AutoWatchOnChanges {
		return nil
	}
	return autoWatch(ctx, userID, repoID)
}

// WatchIfAutoWatchNewRepos subscribes to repo if AutoWatchNewRepos is set
func WatchIfAutoWatchNewRepos(ctx context.Context, userID, repoID int64) error {
	if !setting.Service.AutoWatchNewRepos {
		return nil
	}
	return autoWatch(ctx, userID, repoID)
}

// UnwatchRepos will unwatch the user from all given repositories.
func UnwatchRepos(ctx context.Context, userID int64, repoIDs []int64) error {
	// Unfortunatly, we can't simply delete the Watch records because we do watcher counting in the repo relation.
	for _, repoID := range repoIDs {
		var watch Watch
		var err error
		if watch, err = GetWatch(ctx, userID, repoID); err != nil {
			return err
		}
		// This is a lot simpler than it could be.
		// E.g. when a user explicitly watches a repo and gets blocked, they now explicitly unwatch the repo.
		// That means that when the user won't receive the watch status automatically later on, even when they've been unblocked again.
		// I think this is not too big a problem to add more logic here.
		err = updateWatchRepo(ctx, watch, WatchSourceExplicit, WatchNoneSelection)
		if err != nil {
			return err
		}
	}
	return nil
}
