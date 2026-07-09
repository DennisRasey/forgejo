// Copyright 2025 The Forgejo Authors.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"context"
	"testing"

	"forgejo.org/models/db"
	migration_tests "forgejo.org/models/gitea_migrations/test"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type WatchSelection struct {
	Issues       bool
	PullRequests bool
	Releases     bool
}

// Watch is connection request for receiving repository notification.
type Watch struct {
	ID     int64       `xorm:"pk autoincr"`
	UserID int64       `xorm:"UNIQUE(watch)"`
	RepoID int64       `xorm:"UNIQUE(watch)"`
	Source WatchSource `xorm:"BOOL DEFAULT TRUE"`
	// In the next PR there will be another mode here, choosing the user preset or a custom selection.

	WatchSelectionIssues       bool `xorm:"BOOL DEFAULT TRUE"`
	WatchSelectionPullRequests bool `xorm:"BOOL DEFAULT TRUE"`
	WatchSelectionReleases     bool `xorm:"BOOL DEFAULT TRUE"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// Warning: this does not set the WatchMode.
// The caller needs to do that properly.
func (w *Watch) setWatchSelection(watchSelection WatchSelection) {
	w.WatchSelectionIssues = watchSelection.Issues
	w.WatchSelectionPullRequests = watchSelection.PullRequests
	w.WatchSelectionReleases = watchSelection.Releases
}

var (
	WatchAllSelection  = WatchSelection{Issues: true, PullRequests: true, Releases: true}
	WatchNoneSelection = WatchSelection{Issues: false, PullRequests: false, Releases: false}
)

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

func Test_addGranularWatchColumnsAndDropModeColumn(t *testing.T) {
	// copy of old code //
	type WatchMode uint8
	type Watch struct {
		ID     int64     `xorm:"pk autoincr"`
		UserID int64     `xorm:"UNIQUE(watch)"`
		RepoID int64     `xorm:"UNIQUE(watch)"`
		Mode   WatchMode `xorm:"SMALLINT NOT NULL DEFAULT 1"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	// end copy of old code //

	// Prepare TestEnv
	x, deferable := migration_tests.PrepareTestEnv(t, 0,
		new(Watch),
	)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// test for expected results
	getColumn := func(tn, co string) *schemas.Column {
		tables, err := x.DBMetas()
		require.NoError(t, err)
		var table *schemas.Table
		for _, elem := range tables {
			if elem.Name == tn {
				table = elem
				break
			}
		}
		return table.GetColumn(co)
	}

	require.NotNil(t, getColumn("watch", "mode"))
	cnt1, err := x.Table("watch").Count()
	require.NoError(t, err)
	require.Equal(t, int64(7), cnt1)

	require.NoError(t, addGranularWatchColumnsAndDropModeColumn(x))

	require.Nil(t, getColumn("watch", "mode"))
	require.NotNil(t, getColumn("watch", "watch_selection_issues"))
	require.NotNil(t, getColumn("watch", "watch_selection_pull_requests"))
	require.NotNil(t, getColumn("watch", "watch_selection_releases"))
	cnt2, err := x.Table("watch").Count()
	require.NoError(t, err)
	require.Equal(t, int64(7), cnt2)

	{
		watch, err := GetWatch(db.DefaultContext, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, WatchSourceExplicit, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.True(t, watch.WatchSelectionPullRequests)
		assert.True(t, watch.WatchSelectionReleases)
	}
	{
		watch, err := GetWatch(db.DefaultContext, 4, 1)
		require.NoError(t, err)
		assert.Equal(t, WatchSourceExplicit, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.True(t, watch.WatchSelectionPullRequests)
		assert.True(t, watch.WatchSelectionReleases)
	}
	{
		watch, err := GetWatch(db.DefaultContext, 9, 1)
		require.NoError(t, err)
		assert.Equal(t, WatchSourceExplicit, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.True(t, watch.WatchSelectionPullRequests)
		assert.True(t, watch.WatchSelectionReleases)
	}
	{
		watch, err := GetWatch(db.DefaultContext, 8, 1)
		require.NoError(t, err)
		assert.Equal(t, WatchSourceExplicit, watch.Source)
		assert.False(t, watch.WatchSelectionIssues)
		assert.False(t, watch.WatchSelectionPullRequests)
		assert.False(t, watch.WatchSelectionReleases)
	}
	{
		watch, err := GetWatch(db.DefaultContext, 11, 1)
		require.NoError(t, err)
		assert.Equal(t, WatchSourceAutomatic, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.True(t, watch.WatchSelectionPullRequests)
		assert.True(t, watch.WatchSelectionReleases)
	}
	{
		watch, err := GetWatch(db.DefaultContext, 4, 2)
		require.NoError(t, err)
		assert.Equal(t, WatchSourceExplicit, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.True(t, watch.WatchSelectionPullRequests)
		assert.True(t, watch.WatchSelectionReleases)
	}
	{
		watch, err := GetWatch(db.DefaultContext, 5, 2)
		require.NoError(t, err)
		assert.Equal(t, WatchSourceAutomatic, watch.Source)
		assert.False(t, watch.WatchSelectionIssues)
		assert.False(t, watch.WatchSelectionPullRequests)
		assert.False(t, watch.WatchSelectionReleases)
	}
}
