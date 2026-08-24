// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"fmt"
	"testing"

	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/services/funding"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	config1 = "custom: test.local\n"
	config2 = "ko_fi: test\n"
)

func assertEntriesMatchConfig1(t *testing.T, repo *repo_model.Repository, fnd *funding.RepoFunding, path string) {
	assert.Equal(t, fmt.Sprintf("%s/src/branch/main/%s", repo.Link(), path), fnd.ConfigPath)
	assert.Empty(t, fnd.Errors)
	assert.Len(t, fnd.Entries, 1)
	entry := fnd.Entries[0]
	assert.Equal(t, "custom", entry.ProviderName)
	assert.Equal(t, "https://test.local", entry.Title)
	assert.Equal(t, "https://test.local", entry.Value)
}

// This is called from tests/integration/funding_retrievers_test.go
//
// For coverage:
//   - At the root of the source tree
//   - `COVERAGE_TEST_ARGS='-test.v -test.run=TestFundingRetrieval' make coverage-reset coverage-run-integration-sqlite coverage-show-percentage | grep services/funding/funding.go | sed -e 's/\t\t*/ /g'`
//   - `uncover coverage/textfmt.out GetFundingFromDefaultBranch` etc.
func FromDefaultBranch(t *testing.T) {
	// only a few basic cases, to demonstrate case insensitivity
	paths := []string{
		"funding.yml",
		"Funding.yml",
		"Funding.yaml",
		".github/FUNDING.yml",
		".forgejo/funding.yaml",
	}
	subURLs := []string{
		"/",
		"/test",
		"/test/again",
	}

	t.Run("empty repo", func(t *testing.T) {
		repo := forgery.CreateRepository(t, nil, nil)

		fnd, err := funding.GetFundingFromDefaultBranch(t.Context(), repo)
		require.Nil(t, fnd)
		require.Error(t, err)
		assert.ErrorIs(t, err, funding.ErrFundingNotExist{})
	})

	t.Run("init repo", func(t *testing.T) {
		repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
			Files: forgery.FilesInit{},
		})

		fnd, err := funding.GetFundingFromDefaultBranch(t.Context(), repo)
		require.Nil(t, fnd)
		require.Error(t, err)
		assert.ErrorIs(t, err, funding.ErrFundingNotExist{})
	})

	for _, subURL := range subURLs {
		for _, path := range paths {
			t.Run("repo at "+subURL+" with funding at "+path, func(t *testing.T) {
				defer test.MockVariableValue(&setting.AppSubURL, subURL)() // ensures the ConfigPath works reliably at any URL prefix

				repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
					Files: forgery.MapFS{path: forgery.MapFile(config1)},
				})

				fnd, err := funding.GetFundingFromDefaultBranch(t.Context(), repo)
				require.NoError(t, err)
				require.NotNil(t, fnd)
				assertEntriesMatchConfig1(t, repo, fnd, path)
			})
		}
	}

	t.Run("prefers .forgejo over .github", func(t *testing.T) {
		repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{
				".forgejo/funding.yml": forgery.MapFile(config1),
				".github/funding.yml":  forgery.MapFile(config2),
			},
		})

		fnd, err := funding.GetFundingFromDefaultBranch(t.Context(), repo)
		require.NoError(t, err)
		require.NotNil(t, fnd)
		assertEntriesMatchConfig1(t, repo, fnd, ".forgejo/funding.yml")
	})

	t.Run("prefers .github over root", func(t *testing.T) {
		repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{
				".github/funding.yml": forgery.MapFile(config1),
				"funding.yml":         forgery.MapFile(config2),
			},
		})

		fnd, err := funding.GetFundingFromDefaultBranch(t.Context(), repo)
		require.NoError(t, err)
		require.NotNil(t, fnd)
		assertEntriesMatchConfig1(t, repo, fnd, ".github/funding.yml")
	})

	t.Run("prefers .forgejo over root", func(t *testing.T) {
		repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{
				".forgejo/funding.yml": forgery.MapFile(config1),
				"funding.yml":          forgery.MapFile(config2),
			},
		})

		fnd, err := funding.GetFundingFromDefaultBranch(t.Context(), repo)
		require.NoError(t, err)
		require.NotNil(t, fnd)
		assertEntriesMatchConfig1(t, repo, fnd, ".forgejo/funding.yml")
	})
}
