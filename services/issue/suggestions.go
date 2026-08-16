// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"strconv"

	issues_model "forgejo.org/models/issues"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/structs"
)

func GetSuggestions(ctx context.Context, repo *repo_model.Repository, isPull optional.Option[bool], keyword string) ([]*structs.IssueSuggestion, error) {
	var issues issues_model.IssueList
	var err error
	pageSize := 5
	if keyword == "" {
		issues, err = issues_model.FindLatestUpdatedIssues(ctx, repo.ID, isPull, pageSize)
		if err != nil {
			return nil, err
		}
	} else {
		indexKeyword, _ := strconv.ParseInt(keyword, 10, 64)
		var issueByIndex *issues_model.Issue
		var excludedID optional.Option[int64]
		if indexKeyword > 0 {
			issueByIndex, err = issues_model.GetIssueByIndex(ctx, repo.ID, indexKeyword)
			if err != nil && !issues_model.IsErrIssueNotExist(err) {
				return nil, err
			}
			if issueByIndex != nil {
				excludedID = optional.Some(issueByIndex.ID)
				pageSize--
			}
		}

		issues, err = issues_model.FindIssuesSuggestionByKeyword(ctx, repo.ID, keyword, isPull, excludedID, pageSize)
		if err != nil {
			return nil, err
		}

		if issueByIndex != nil {
			issues = append([]*issues_model.Issue{issueByIndex}, issues...)
		}
	}

	if err := issues.LoadPullRequests(ctx); err != nil {
		return nil, err
	}

	suggestions := make([]*structs.IssueSuggestion, 0, len(issues))
	for _, issue := range issues {
		if has, value := isPull.Get(); has {
			if issue.IsPull != value {
				continue
			}
		}

		suggestion := &structs.IssueSuggestion{
			Index: issue.Index,
			Title: issue.Title,
			State: issue.State(),
		}

		if issue.IsPull && issue.PullRequest != nil {
			suggestion.IsPr = true
			suggestion.HasMerged = issue.PullRequest.HasMerged
			suggestion.IsWorkInProgress = issue.PullRequest.IsWorkInProgress(ctx)
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}
