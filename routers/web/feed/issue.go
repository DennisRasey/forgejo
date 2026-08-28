// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package feed

import (
	"fmt"
	"time"

	activities_model "forgejo.org/models/activities"
	issues_model "forgejo.org/models/issues"
	"forgejo.org/services/context"

	"github.com/gorilla/feeds"
)

// ShowIssueFeed shows comment activity on the issue as RSS / Atom feed
func ShowIssueFeed(ctx *context.Context, issue *issues_model.Issue, formatType string) {
	actions, _, err := activities_model.GetFeeds(ctx, activities_model.GetFeedsOptions{
		RequestedIssue:       issue,
		Actor:                ctx.Doer,
		IncludePrivate:       true,
		Date:                 ctx.FormString("date"),
		OnlyPerformedByActor: true,
	})
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}

	reference := fmt.Sprintf("%s/%s#%d", issue.Repo.OwnerName, issue.Repo.Name, issue.Index)
	title := ctx.Locale.TrString("repo.rss.issue_feed_title", reference, issue.Title)
	description := ctx.Locale.TrString("repo.rss.updates_on_issue", reference, issue.Poster.DisplayName())

	if issue.IsPull {
		reference = fmt.Sprintf("%s/%s!%d", issue.Repo.OwnerName, issue.Repo.Name, issue.Index)
		title = ctx.Locale.TrString("repo.rss.pull_feed_title", reference, issue.Title)
		description = ctx.Locale.TrString("repo.rss.updates_on_pull", reference, issue.Poster.DisplayName())
	}

	feed := &feeds.Feed{
		Title:       title,
		Link:        &feeds.Link{Href: issue.HTMLURL()},
		Description: description,
		Created:     time.Now(),
	}

	feed.Items, err = feedActionsToFeedItems(ctx, actions)
	if err != nil {
		ctx.ServerError("convert feed", err)
		return
	}

	writeFeed(ctx, feed, formatType)
}
