// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package pull

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"forgejo.org/models"
	"forgejo.org/models/db"
	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unit"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/log"
	"forgejo.org/modules/timeutil"
	notify_service "forgejo.org/services/notify"
)

// MergedManually mark pr as merged manually
func MergedManually(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, baseGitRepo *git.Repository, commitID string) error {
	defer pullWorkingPool.Lock(fmt.Sprint(pr.ID))()

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		if err := pr.LoadBaseRepo(ctx); err != nil {
			return err
		}
		prUnit, err := pr.BaseRepo.GetUnit(ctx, unit.TypePullRequests)
		if err != nil {
			return err
		}
		prConfig := prUnit.PullRequestsConfig()

		// Check if merge style is correct and allowed
		if !prConfig.IsMergeStyleAllowed(repo_model.MergeStyleManuallyMerged) {
			return models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: repo_model.MergeStyleManuallyMerged}
		}

		// Accept full or abbreviated commit IDs, any case. Lowercase first so
		// IsValid's [0-9a-f]{4..FullLength} regex accepts uppercase input;
		// GetCommit then resolves the short form and rejects ambiguous prefixes.
		objectFormat := git.ObjectFormatFromName(pr.BaseRepo.ObjectFormatName)
		commitID = strings.TrimSpace(commitID)
		commitID = strings.ToLower(commitID)
		if !objectFormat.IsValid(commitID) {
			return errors.New("Wrong commit ID")
		}

		commit, err := baseGitRepo.GetCommit(commitID)
		if err != nil {
			if git.IsErrNotExist(err) {
				return errors.New("Wrong commit ID")
			}
			return err
		}
		commitID = commit.ID.String()

		ok, err := baseGitRepo.IsCommitInBranch(commitID, pr.BaseBranch)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("Wrong commit ID")
		}

		pr.MergedCommitID = commitID
		pr.MergedUnix = timeutil.TimeStamp(commit.Author.When.Unix())
		pr.Status = issues_model.PullRequestStatusManuallyMerged
		pr.Merger = doer
		pr.MergerID = doer.ID

		var merged bool
		if merged, err = pr.SetMerged(ctx); err != nil {
			return err
		} else if !merged {
			return errors.New("SetMerged failed")
		}
		return nil
	}); err != nil {
		return err
	}

	notify_service.MergePullRequest(baseGitRepo.Ctx, doer, pr)
	log.Info("manuallyMerged[%d]: Marked as manually merged into %s/%s by commit id: %s", pr.ID, pr.BaseRepo.Name, pr.BaseBranch, commitID)

	return handleCloseCrossReferences(ctx, pr, doer)
}

// manuallyMerged checks if a pull request got manually merged
// When a pull request got manually merged mark the pull request as merged
func manuallyMerged(ctx context.Context, pr *issues_model.PullRequest) bool {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("%-v LoadBaseRepo: %v", pr, err)
		return false
	}

	if unit, err := pr.BaseRepo.GetUnit(ctx, unit.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		if !config.AutodetectManualMerge {
			return false
		}
	} else {
		log.Error("%-v BaseRepo.GetUnit(unit.TypePullRequests): %v", pr, err)
		return false
	}

	commit, err := getMergeCommit(ctx, pr)
	if err != nil {
		log.Error("%-v getMergeCommit: %v", pr, err)
		return false
	}

	if commit == nil {
		// no merge commit found
		return false
	}

	pr.MergedCommitID = commit.ID.String()
	pr.MergedUnix = timeutil.TimeStamp(commit.Author.When.Unix())
	pr.Status = issues_model.PullRequestStatusManuallyMerged
	merger, _ := user_model.GetUserByEmail(ctx, commit.Author.Email)

	// When the commit author is unknown set the BaseRepo owner as merger
	if merger == nil {
		if pr.BaseRepo.Owner == nil {
			if err = pr.BaseRepo.LoadOwner(ctx); err != nil {
				log.Error("%-v BaseRepo.LoadOwner: %v", pr, err)
				return false
			}
		}
		merger = pr.BaseRepo.Owner
	}
	pr.Merger = merger
	pr.MergerID = merger.ID

	if merged, err := pr.SetMerged(ctx); err != nil {
		log.Error("%-v setMerged : %v", pr, err)
		return false
	} else if !merged {
		return false
	}

	notify_service.MergePullRequest(ctx, merger, pr)

	log.Info("manuallyMerged[%-v]: Marked as manually merged into %s/%s by commit id: %s", pr, pr.BaseRepo.Name, pr.BaseBranch, commit.ID.String())
	return true
}
