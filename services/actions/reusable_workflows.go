// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"fmt"
	"io"

	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/gitrepo"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
	"code.forgejo.org/forgejo/runner/v13/act/model"
)

type CleanupFunc func()

// Evaluate whether we want to expand reusable workflows to their internal workflows. If the job has defined `runs-on`
// labels, then we will not expand them -- this maintains the legacy behaviour from before reusable workflow expansion.
// If `runs-on` is absent then we will attempt to expand the job.
func expandForJob(job *jobparser.Job) bool {
	return len(job.RunsOn()) == 0
}

// Provide a closure for `jobparser.ExpandLocalReusableWorkflows` which resolves reusable workflow references local to
// the given commit.  A reusable workflow reference is a job with a `uses: ./.forgejo/workflows/some-path.yaml`, and
// resolving it involves reading the target file in the target commit and returning the file contents.
//
// See `expandForJob` for information about jobs that are exempt from expansion.
var expandLocalReusableWorkflows = func(commit *git.Commit) jobparser.LocalWorkflowFetcher {
	return func(job *jobparser.Job, path string) ([]byte, error) {
		if !expandForJob(job) {
			return nil, jobparser.ErrUnsupportedReusableWorkflowFetch
		}

		blob, err := commit.GetBlobByPath(path)
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to access path %s: %w", path, err)
		}

		reader, err := blob.DataAsync()
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to read path %s: %w", path, err)
		}

		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to read path %s: %w", path, err)
		}

		return content, nil
	}
}

// Provide a closure for `jobparser.ExpandLocalReusableWorkflows` which resolves reusable workflow references local to
// the given repo & commit SHA.  This variation lazily opens the target git repo and reads the commit only when a local
// reusable workflow is needed, and then caches the commit if multiple workflows need to be read.  A cleanup function is
// also returned that will close the open git repo, and should be `defer` executed.
//
// See `expandForJob` for information about jobs that are exempt from expansion.
var lazyRepoExpandLocalReusableWorkflow = func(ctx context.Context, repoID int64, commitSHA string) (jobparser.LocalWorkflowFetcher, CleanupFunc) {
	// In the event that local reusable workflows (eg. `uses: ./.forgejo/workflows/reusable.yml`) are present, we'll
	// need to read the commit of the repo to resolve that reference. But most workflows don't do this, so save the
	// effort of opening the git repo and fetching the schedule's `CommitSHA` commit if it's not necessary by wrapping
	// that logic in a caching closure, `getGitCommit`.
	var innerFetcher jobparser.LocalWorkflowFetcher
	var gitRepo *git.Repository
	cleanupFunc := func() {
		if gitRepo != nil {
			gitRepo.Close()
		}
	}
	fetcher := func(job *jobparser.Job, path string) ([]byte, error) {
		if !expandForJob(job) {
			return nil, jobparser.ErrUnsupportedReusableWorkflowFetch
		}
		if innerFetcher != nil {
			content, err := innerFetcher(job, path)
			return content, err
		}
		repo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to get repo: %w", err)
		}
		gitRepo, err = gitrepo.OpenRepository(ctx, repo) // ensure this keeps reference to the outer closure's `gitRepo`, not a local definition
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to open repo: %w", err)
		}
		commit, err := gitRepo.GetCommit(commitSHA)
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to open commit %q on repo %s: %w", commitSHA, repo.FullName(), err)
		}
		innerFetcher = expandLocalReusableWorkflows(commit)
		content, err := innerFetcher(job, path)
		return content, err
	}
	return fetcher, cleanupFunc
}

// Standard function for `jobparser.ExpandInstanceReusableWorkflows` which resolves reusable workflow references on the
// same Forgejo instance, but in a specific repo. For example, `uses:
// some-org/some-repo/.forgejo/workflows/some-path.yaml@ref`. Resolving it involves reading the target file in the
// target repo & commit and returning the file contents.
//
// See `expandForJob` for information about jobs that are exempt from expansion.
var expandInstanceReusableWorkflows = func(ctx context.Context) jobparser.InstanceWorkflowFetcher {
	return func(job *jobparser.Job, ref *model.NonLocalReusableWorkflowReference) ([]byte, error) {
		if !expandForJob(job) {
			return nil, jobparser.ErrUnsupportedReusableWorkflowFetch
		}

		owner, err := user_model.GetUserByName(ctx, ref.Org)
		// Reusable workflows don't currently support access to any private repos -- that's implemented here as well,
		// although in the future it might be possible to use context information about the executing workflow to
		// broaden access (eg. repos within an org could access other repos within the same org, perhaps).
		if (err != nil && user_model.IsErrUserNotExist(err)) || !owner.Visibility.IsPublic() {
			// Same error message is returned for non-existing & non-visible to avoid information leak
			return nil, fmt.Errorf("expanding reusable workflow failed to access user %s: user does not exist", ref.Org)
		} else if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to access user %s: %w", ref.Org, err)
		}

		repo, err := repo_model.GetRepositoryByName(ctx, owner.ID, ref.Repo)
		if (err != nil && repo_model.IsErrRepoNotExist(err)) || repo.IsPrivate {
			// Same error message is returned for non-existing & non-visible to avoid information leak
			return nil, fmt.Errorf("expanding reusable workflow failed to access repo %s: repo does not exist", ref.Repo)
		} else if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to access repo %s: %w", ref.Repo, err)
		}

		gitRepo, err := gitrepo.OpenRepository(ctx, repo)
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to open repo %s: %w", repo.FullName(), err)
		}
		defer gitRepo.Close()

		commitID, err := gitRepo.GetRefCommitID(ref.Ref)
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to resolve reference %q on repo %s: %w", ref.Ref, repo.FullName(), err)
		}

		commit, err := gitRepo.GetCommit(commitID)
		if err != nil {
			return nil, fmt.Errorf("expanding reusable workflow failed to open commit %q on repo %s: %w", commitID, repo.FullName(), err)
		}

		data, err := expandLocalReusableWorkflows(commit)(job, ref.FilePath())
		return data, err
	}
}
