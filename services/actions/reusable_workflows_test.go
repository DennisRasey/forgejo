// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/git"
	"forgejo.org/modules/setting"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
	"code.forgejo.org/forgejo/runner/v13/act/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

const testWorkflow string = `on:
  workflow_call:
    inputs:
      example-string-required:
        required: true
        type: string

name: test
jobs:
  job1:
    name: "job1 (local)"
    runs-on: ubuntu-slim
    steps:
      - name: Echo inputs
        run: |
          echo example-string-required="${{ inputs.example-string-required }}"

`

func TestExpandForJob(t *testing.T) {
	job := jobparser.Job{}

	err := yaml.Unmarshal([]byte("{ name: job1 }"), &job)
	require.NoError(t, err)
	assert.True(t, expandForJob(&job))

	err = yaml.Unmarshal([]byte("{ name: job1, runs-on: ubuntu-latest }"), &job)
	require.NoError(t, err)
	assert.False(t, expandForJob(&job))

	err = yaml.Unmarshal([]byte("{ name: job1, runs-on: [x64, ubuntu-latest] }"), &job)
	require.NoError(t, err)
	assert.False(t, expandForJob(&job))
}

func TestExpandLocalReusableWorkflows(t *testing.T) {
	gitRepo, err := git.OpenRepository(git.DefaultContext, "./TestExpandLocalReusableWorkflows")
	require.NoError(t, err)
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit("0a621162c63e842226816407805f3ca4dfc4c8bc")
	require.NoError(t, err)

	fetcher := expandLocalReusableWorkflows(commit)
	require.NotNil(t, fetcher)

	t.Run("successful fetch", func(t *testing.T) {
		content, err := fetcher(&jobparser.Job{}, "./.forgejo/workflows/reusable-1.yml")
		require.NoError(t, err)
		assert.Equal(t, testWorkflow, string(content))
	})

	// modules/git/blob.go has a specific optimized codepath for reading files <4096 bytes, where all data is buffered
	// into memory immediately.  In that case, the reader doesn't need to be closed since it's a `NopCloser`, but in all
	// other cases it does.  This test verifies that >4k files can be read without errors.
	t.Run("successful fetch > 4k", func(t *testing.T) {
		content, err := fetcher(&jobparser.Job{}, "./.forgejo/workflows/reusable-more-than-4k.yml")
		require.NoError(t, err)
		assert.Greater(t, len(content), 4096)
	})

	t.Run("file not exist", func(t *testing.T) {
		_, err = fetcher(&jobparser.Job{}, "./forgejo/workflows/reusable-2.yml")
		require.ErrorContains(t, err, "expanding reusable workflow failed to access path ./forgejo/workflows/reusable-2.yml: object does not exist")
	})

	t.Run("do not expand due to runs-on", func(t *testing.T) {
		jobWithRunsOn := jobparser.Job{}
		err = yaml.Unmarshal([]byte("{ name: job1, runs-on: ubuntu-latest }"), &jobWithRunsOn)
		require.NoError(t, err)
		assert.False(t, expandForJob(&jobWithRunsOn))
		_, err = fetcher(&jobWithRunsOn, "./.forgejo/workflows/reusable-1.yml")
		require.ErrorIs(t, jobparser.ErrUnsupportedReusableWorkflowFetch, err)
	})
}

func replaceTestRepo(t *testing.T, owner, repo, replacement string) {
	t.Helper()

	// Copy the repository into the target path that `gitrepo.OpenRepository` will look for it.
	repoPath := filepath.Join(setting.RepoRootPath, strings.ToLower(owner), strings.ToLower(repo)+".git")
	err := os.RemoveAll(repoPath) // there's a default repo copied here by the fixture setup that we want to replace
	require.NoError(t, err)
	err = os.CopyFS(repoPath, os.DirFS(replacement))
	require.NoError(t, err)
}

func TestLazyRepoExpandLocalReusableWorkflows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Shouldn't need valid content if we never call the lazy evaluator
	lazy1, cleanup := lazyRepoExpandLocalReusableWorkflow(t.Context(), -123456, "this is not a valid commit SHA")
	assert.NotNil(t, lazy1)
	assert.NotNil(t, cleanup)
	cleanup()

	replaceTestRepo(t, "user2", "repo1", "./TestExpandLocalReusableWorkflows")

	lazy2, cleanup := lazyRepoExpandLocalReusableWorkflow(t.Context(), 1, "e3868ecb4f8b483fc0bdd422561bf0062a7df907")
	assert.NotNil(t, lazy2)
	assert.NotNil(t, cleanup)
	content, err := lazy2(&jobparser.Job{}, "./.forgejo/workflows/reusable-1.yml")
	require.NoError(t, err)
	assert.Equal(t, testWorkflow, string(content))
	cleanup()
}

func TestExpandInstanceReusableWorkflows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	tests := []struct {
		name          string
		ref           *model.NonLocalReusableWorkflowReference
		errIs         error
		errorContains string
		repo          string
		hasRunsOn     bool
	}{
		{
			name:      "hasRunsOn",
			hasRunsOn: true,
			ref:       &model.NonLocalReusableWorkflowReference{},
			errIs:     jobparser.ErrUnsupportedReusableWorkflowFetch,
		},
		{
			name: "non-existent owner",
			ref: &model.NonLocalReusableWorkflowReference{
				Org: "owner-does-not-exist",
			},
			errorContains: "owner-does-not-exist: user does not exist",
		},
		{
			name: "non-public owner",
			ref: &model.NonLocalReusableWorkflowReference{
				Org: "user33",
			},
			errorContains: "user33: user does not exist",
		},
		{
			name: "non-existent repo",
			ref: &model.NonLocalReusableWorkflowReference{
				Org:  "user2",
				Repo: "repo10000",
			},
			errorContains: "repo10000: repo does not exist",
		},
		{
			name: "non-public repo",
			ref: &model.NonLocalReusableWorkflowReference{
				Org:  "user2",
				Repo: "repo2",
			},
			errorContains: "repo2: repo does not exist",
		},
		{
			name: "public repo",
			ref: &model.NonLocalReusableWorkflowReference{
				Org:         "user2",
				Repo:        "repo1",
				GitPlatform: "forgejo",
				Filename:    "reusable-1.yml",
				Ref:         "main",
			},
			repo: "./TestExpandLocalReusableWorkflows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.repo != "" {
				replaceTestRepo(t, tt.ref.Org, tt.ref.Repo, tt.repo)
			}

			job := jobparser.Job{}
			if tt.hasRunsOn {
				err := yaml.Unmarshal([]byte("{ name: job1, runs-on: ubuntu-latest }"), &job)
				require.NoError(t, err)
			}
			fetcher := expandInstanceReusableWorkflows(t.Context())
			content, err := fetcher(&job, tt.ref)
			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
			} else if tt.errorContains != "" {
				require.ErrorContains(t, err, tt.errorContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testWorkflow, string(content))
			}
		})
	}
}
