// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsWatching(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 1, 1))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 4, 1))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 11, 1))

	assert.False(t, repo_model.IsWatcher(db.DefaultContext, 1, 5))
	assert.False(t, repo_model.IsWatcher(db.DefaultContext, 8, 1))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 4, 3))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 4, 4))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 4, 5))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 5, 5))
	watch, err := repo_model.GetWatch(db.DefaultContext, 5, 5)
	require.NoError(t, err)
	assert.Equal(t, repo_model.WatchSourceExplicit, watch.Source)
	assert.False(t, watch.WatchSelectionIssues)
	assert.True(t, watch.WatchSelectionPullRequests)
	assert.True(t, watch.WatchSelectionReleases)

	assert.False(t, repo_model.IsWatcher(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID))
}

func TestGetSelectWatchers(t *testing.T) {
	for idx, test := range []struct {
		RepoID          int64
		Selection       repo_model.WatchSelection
		ExpectedUserIDs []int64
	}{
		{
			1,
			repo_model.WatchNoneSelection,
			// This should always be empty for any repo.
			[]int64{},
		},
		{
			3,
			repo_model.WatchNoneSelection,
			// This should always be empty for any repo.
			[]int64{},
		},
		{
			3,
			repo_model.WatchAllSelection,
			[]int64{4},
		},
		{
			3,
			repo_model.WatchSelection{Issues: true},
			[]int64{4},
		},
		{
			3,
			repo_model.WatchSelection{PullRequests: true},
			[]int64{},
		},
		{
			3,
			repo_model.WatchSelection{Releases: true},
			[]int64{},
		},
		{
			3,
			repo_model.WatchSelection{Releases: true, PullRequests: true},
			[]int64{},
		},

		{
			5,
			repo_model.WatchSelection{Releases: true},
			[]int64{4, 5},
		},
		{
			5,
			repo_model.WatchSelection{Releases: true, PullRequests: true},
			[]int64{4, 5},
		},
		{
			5,
			repo_model.WatchSelection{PullRequests: true},
			[]int64{5},
		},
	} {
		watchers, err := repo_model.GetSelectWatchers(db.DefaultContext, test.RepoID, test.Selection)
		require.NoError(t, err)
		if assert.Len(t, watchers, len(test.ExpectedUserIDs), "idx: %d", idx) {
			for i, watcher := range watchers {
				assert.Equal(t, test.ExpectedUserIDs[i], watcher.UserID)
			}
		}

		watcherIDs, err := repo_model.GetSelectWatcherIDs(db.DefaultContext, test.RepoID, test.Selection)
		require.NoError(t, err)
		assert.Equal(t, test.ExpectedUserIDs, watcherIDs, "idx: %d", idx)
	}
}

func TestRepository_GetWatchers(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watchers, err := repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)
	for _, watcher := range watchers {
		unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: watcher.ID, RepoID: repo.ID})
	}

	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 9})
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Empty(t, watchers)
}

func TestWatchRepoExplicitly(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	// There is no record for this watch in the fixture.
	assert.False(t, repo_model.IsWatcher(db.DefaultContext, 1, 2))
	{
		watch, err := repo_model.GetWatch(db.DefaultContext, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, repo_model.WatchSourceAutomatic, watch.Source)
		assert.False(t, watch.WatchSelectionIssues)
		assert.False(t, watch.WatchSelectionPullRequests)
		assert.False(t, watch.WatchSelectionReleases)
	}

	require.NoError(t, repo_model.WatchRepoExplicitly(db.DefaultContext, 1, 2, repo_model.WatchAllSelection))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 1, 2))
	{
		watch, err := repo_model.GetWatch(db.DefaultContext, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, repo_model.WatchSourceExplicit, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.True(t, watch.WatchSelectionPullRequests)
		assert.True(t, watch.WatchSelectionReleases)
	}

	require.NoError(t, repo_model.WatchRepoExplicitly(db.DefaultContext, 1, 2, repo_model.WatchNoneSelection))
	assert.False(t, repo_model.IsWatcher(db.DefaultContext, 1, 2))
	{
		watch, err := repo_model.GetWatch(db.DefaultContext, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, repo_model.WatchSourceExplicit, watch.Source)
		assert.False(t, watch.WatchSelectionIssues)
		assert.False(t, watch.WatchSelectionPullRequests)
		assert.False(t, watch.WatchSelectionReleases)
	}

	require.NoError(t, repo_model.WatchRepoExplicitly(db.DefaultContext, 1, 2, repo_model.WatchSelection{Issues: true}))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 1, 2))
	{
		watch, err := repo_model.GetWatch(db.DefaultContext, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, repo_model.WatchSourceExplicit, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.False(t, watch.WatchSelectionPullRequests)
		assert.False(t, watch.WatchSelectionReleases)
	}

	require.NoError(t, repo_model.WatchRepoExplicitly(db.DefaultContext, 1, 2, repo_model.WatchSelection{Issues: true, PullRequests: true}))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 1, 2))
	{
		watch, err := repo_model.GetWatch(db.DefaultContext, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, repo_model.WatchSourceExplicit, watch.Source)
		assert.True(t, watch.WatchSelectionIssues)
		assert.True(t, watch.WatchSelectionPullRequests)
		assert.False(t, watch.WatchSelectionReleases)
	}

	require.NoError(t, repo_model.WatchRepoExplicitly(db.DefaultContext, 1, 2, repo_model.WatchSelection{Releases: true}))
	assert.True(t, repo_model.IsWatcher(db.DefaultContext, 1, 2))
	{
		watch, err := repo_model.GetWatch(db.DefaultContext, 1, 2)
		require.NoError(t, err)
		assert.Equal(t, repo_model.WatchSourceExplicit, watch.Source)
		assert.False(t, watch.WatchSelectionIssues)
		assert.False(t, watch.WatchSelectionPullRequests)
		assert.True(t, watch.WatchSelectionReleases)
	}
}

func TestWatchIfAutoWatchNewRepos(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watchers, err := repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)

	setting.Service.AutoWatchNewRepos = false

	prevCount := repo.NumWatches

	// Must not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchNewRepos(db.DefaultContext, 8, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchNewRepos(db.DefaultContext, 10, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	setting.Service.AutoWatchNewRepos = true

	// Must not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchNewRepos(db.DefaultContext, 8, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	// We simply don't WatchIfAuto
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should add watch
	require.NoError(t, repo_model.WatchIfAutoWatchNewRepos(db.DefaultContext, 12, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount+1)
	watch, err := repo_model.GetWatch(db.DefaultContext, 12, 1)
	require.NoError(t, err)
	require.Equal(t, repo_model.WatchSourceAutomatic, watch.Source)
	require.True(t, watch.WatchSelectionIssues)
	require.True(t, watch.WatchSelectionPullRequests)
	require.True(t, watch.WatchSelectionReleases)

	// Should remove watch, inhibit from adding auto
	require.NoError(t, repo_model.WatchRepoExplicitly(db.DefaultContext, 12, 1, repo_model.WatchNoneSelection))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Must not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchNewRepos(db.DefaultContext, 12, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)
}

func TestWatchIfAutoWatchOnChanges(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	watchers, err := repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, repo.NumWatches)

	setting.Service.AutoWatchOnChanges = false

	prevCount := repo.NumWatches

	// Must not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchOnChanges(db.DefaultContext, 8, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchOnChanges(db.DefaultContext, 10, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	setting.Service.AutoWatchOnChanges = true

	// Must not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchOnChanges(db.DefaultContext, 8, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should not add watch
	// We simply don't WatchIfAuto
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Should add watch
	require.NoError(t, repo_model.WatchIfAutoWatchOnChanges(db.DefaultContext, 12, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount+1)
	watch, err := repo_model.GetWatch(db.DefaultContext, 12, 1)
	require.NoError(t, err)
	require.Equal(t, repo_model.WatchSourceAutomatic, watch.Source)
	require.True(t, watch.WatchSelectionIssues)
	require.True(t, watch.WatchSelectionPullRequests)
	require.True(t, watch.WatchSelectionReleases)

	// Should remove watch, inhibit from adding auto
	require.NoError(t, repo_model.WatchRepoExplicitly(db.DefaultContext, 12, 1, repo_model.WatchNoneSelection))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)

	// Must not add watch
	require.NoError(t, repo_model.WatchIfAutoWatchOnChanges(db.DefaultContext, 12, 1))
	watchers, err = repo_model.GetRepoWatchers(db.DefaultContext, repo.ID, db.ListOptions{Page: 1})
	require.NoError(t, err)
	assert.Len(t, watchers, prevCount)
}

func TestUnwatchRepos(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: 4, RepoID: 1})
	unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: 4, RepoID: 2})

	err := repo_model.UnwatchRepos(db.DefaultContext, 4, []int64{1, 2})
	require.NoError(t, err)

	assert.Equal(t, 1, unittest.GetCount(t, &repo_model.Watch{UserID: 4, RepoID: 1}, "source = false AND watch_selection_issues = false AND watch_selection_pull_requests = false AND watch_selection_releases = false"))
	assert.Equal(t, 1, unittest.GetCount(t, &repo_model.Watch{UserID: 4, RepoID: 2}, "source = false AND watch_selection_issues = false AND watch_selection_pull_requests = false AND watch_selection_releases = false"))
}
