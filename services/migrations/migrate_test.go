// Copyright 2026 The Forgejo Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"forgejo.org/modules/migration"
	"forgejo.org/services/migrations/allowlist"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ migration.Uploader = nullUploader{}

type nullUploader struct{}

func (nullUploader) MaxBatchInsertSize(string) int {
	return 1
}

func (nullUploader) CreateRepo(*migration.Repository, migration.MigrateOptions) error {
	return nil
}

func (nullUploader) CreateTopics(...string) error {
	return nil
}

func (nullUploader) CreateMilestones(...*migration.Milestone) error {
	return nil
}

func (nullUploader) CreateReleases(...*migration.Release) error {
	return nil
}

func (nullUploader) SyncTags() error {
	return nil
}

func (nullUploader) CreateLabels(...*migration.Label) error {
	return nil
}

func (nullUploader) EnableCommentReplyTo() {}

func (nullUploader) CreateIssues(...*migration.Issue) error {
	return nil
}

func (nullUploader) CreateComments(...*migration.Comment) error {
	return nil
}

func (nullUploader) CreatePullRequests(...*migration.PullRequest) error {
	return nil
}

func (nullUploader) CreateReviews(...*migration.Review) error {
	return nil
}

func (nullUploader) Rollback() error {
	return nil
}

func (nullUploader) Finish() error {
	return nil
}

func (nullUploader) Close() {}

type testingDownloader struct {
	migration.NullDownloader
}

func (testingDownloader) GetRepoInfo() (*migration.Repository, error) {
	return &migration.Repository{
		CloneURL: "https://codeberg.org/forgejo-contrib/delightful-forgejo.git",
	}, nil
}

func (testingDownloader) GetIssues(page, _ int) ([]*migration.Issue, bool, error) {
	return make([]*migration.Issue, 1), page == 2, nil
}

func (testingDownloader) GetPullRequests(page, _ int) ([]*migration.PullRequest, bool, error) {
	return make([]*migration.PullRequest, 1), page == 3, nil
}

func TestMigrateRepository(t *testing.T) {
	messages := []struct {
		key  string
		args []any
	}{
		{key: "migrate.in_progress.git"},
		{key: "migrate.in_progress.topics"},
		{key: "migrate.in_progress.issues"},
		{key: "migrate.in_progress.issues.progress", args: []any{1}},
		{key: "migrate.in_progress.issues.progress", args: []any{2}},
		{key: "migrate.in_progress.pulls"},
		{key: "migrate.in_progress.pulls.progress", args: []any{1}},
		{key: "migrate.in_progress.pulls.progress", args: []any{2}},
		{key: "migrate.in_progress.pulls.progress", args: []any{3}},
	}
	messenger := func(key string, args ...any) {
		if assert.NotEmpty(t, messages, key) {
			assert.Equal(t, messages[0].key, key)
			assert.Equal(t, messages[0].args, args)
			messages = messages[1:]
		}
	}

	require.NoError(t, migrateRepository(nil, nil, testingDownloader{}, nullUploader{}, migration.MigrateOptions{
		PullRequests: true,
		Issues:       true,
	}, messenger))

	assert.Empty(t, messages)
}

func TestMigrationUserAgent(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "Forgejo-migration-http-client/1.1", userAgent)
	}))
	defer testServer.Close()

	client := allowlist.NewMigrationHTTPClient()

	req, err := http.NewRequest("GET", testServer.URL, nil)
	require.NoError(t, err)
	client.Do(req)
}
