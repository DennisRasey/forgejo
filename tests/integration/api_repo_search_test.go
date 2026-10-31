// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3

package integration

import (
	"net/http"
	"net/url"
	"testing"

	api "forgejo.org/modules/structs"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
)

type RepoSearchResults struct {
	OK    bool              `json:"ok"`
	Repos []*api.Repository `json:"data"`
}

func TestAPIRepoSearchLanguage(t *testing.T) {
	const repositoryNamePrefix = "api-search-language-repo-"

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		testRepos := []struct {
			name string // repository name
			file string // file inside the repo
		}{
			{
				name: repositoryNamePrefix + "01go",
				file: "test.go",
			},
			{
				name: repositoryNamePrefix + "02go",
				file: "test.go",
			},
			{
				name: repositoryNamePrefix + "01python",
				file: "test.py",
			},
			{
				name: repositoryNamePrefix + "02python",
				file: "test.py",
			},
			{
				name: repositoryNamePrefix + "03python",
				file: "test.py",
			},
			{
				name: repositoryNamePrefix + "01kotlin",
				file: "test.kt",
			},
		}
		for _, testRepo := range testRepos {
			forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
				Name: testRepo.name,
				Files: forgery.MapFS{
					"repo-search-test-data/" + testRepo.file: forgery.MapFile(`package unknown`),
				},
			},
			)
		}
		testCases := []struct {
			language string
			count    int
		}{
			{
				language: "Kotlin",
				count:    1,
			},
			{
				language: "Go",
				count:    2,
			},
			{
				language: "Python",
				count:    3,
			},
			{
				language: "Lisp",
				count:    0,
			},
			{
				language: "",
				count:    len(testRepos),
			},
		}
		for _, testCase := range testCases {
			t.Run("test-repo-search-language", func(t *testing.T) {
				// The test environment contains repositories from all tests (not just from this file).
				// In order to have predictable results it is necessary to limit the results by searching
				// for the repository prefix in addition.
				request := NewRequestf(t, "GET", "/api/v1/repos/search?language=%s&q=%s",
					testCase.language, repositoryNamePrefix)
				response := MakeRequest(t, request, http.StatusOK)
				var result RepoSearchResults
				DecodeJSON(t, response, &result)
				assert.Len(t,
					result.Repos, testCase.count,
					"There should be %d repositories with language '%s'.", testCase.count, testCase.language)
			})
		}
	},
	)
}
