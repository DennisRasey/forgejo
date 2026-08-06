// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/util"
	webhook_module "forgejo.org/modules/webhook"
)

type (
	linkFormatter = func(url, text string) string
	nameFormatter = func(name string) string
)

// noneLinkFormatter does not create a link but just returns the text
func noneLinkFormatter(url, text string) string {
	return text
}

// htmlLinkFormatter creates a HTML link
func htmlLinkFormatter(url, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(url), html.EscapeString(text))
}

// noneNameFormatter just returns the name
func noneNameFormatter(name string) string {
	return name
}

// getPullRequestInfo gets the information for a pull request
func getPullRequestInfo(p *api.PullRequestPayload, nameFormatter nameFormatter) (title, link, by, operator, operateResult, assignees string) {
	title = fmt.Sprintf("[PullRequest-%s #%d]: %s\n%s", p.Repository.FullName, p.PullRequest.Index, p.Action, p.PullRequest.Title)
	assignList := p.PullRequest.Assignees
	assignStringList := make([]string, len(assignList))

	for i, user := range assignList {
		assignStringList[i] = nameFormatter(user.UserName)
	}
	switch p.Action {
	case api.HookIssueAssigned:
		operateResult = fmt.Sprintf("%s assign this to %s", nameFormatter(p.Sender.UserName), nameFormatter(assignList[len(assignList)-1].UserName))
	case api.HookIssueUnassigned:
		operateResult = fmt.Sprintf("%s unassigned this for someone", nameFormatter(p.Sender.UserName))
	case api.HookIssueMilestoned:
		operateResult = fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.PullRequest.Milestone.ID)
	}
	link = p.PullRequest.HTMLURL
	by = fmt.Sprintf("PullRequest by %s", nameFormatter(p.PullRequest.Poster.UserName))
	if len(assignStringList) > 0 {
		assignees = fmt.Sprintf("Assignees: %s", strings.Join(assignStringList, ", "))
	}
	operator = fmt.Sprintf("Operator: %s", nameFormatter(p.Sender.UserName))
	return title, link, by, operator, operateResult, assignees
}

// getIssuesInfo gets the information for an issue
func getIssuesInfo(p *api.IssuePayload, nameFormatter nameFormatter) (issueTitle, link, by, operator, operateResult, assignees string) {
	issueTitle = fmt.Sprintf("[Issue-%s #%d]: %s\n%s", p.Repository.FullName, p.Issue.Index, p.Action, p.Issue.Title)
	assignList := p.Issue.Assignees
	assignStringList := make([]string, len(assignList))

	for i, user := range assignList {
		assignStringList[i] = nameFormatter(user.UserName)
	}
	switch p.Action {
	case api.HookIssueAssigned:
		operateResult = fmt.Sprintf("%s assign this to %s", nameFormatter(p.Sender.UserName), nameFormatter(assignList[len(assignList)-1].UserName))
	case api.HookIssueUnassigned:
		operateResult = fmt.Sprintf("%s unassigned this for someone", nameFormatter(p.Sender.UserName))
	case api.HookIssueMilestoned:
		operateResult = fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.Issue.Milestone.ID)
	}
	link = p.Issue.HTMLURL
	by = fmt.Sprintf("Issue by %s", nameFormatter(p.Issue.Poster.UserName))
	if len(assignStringList) > 0 {
		assignees = fmt.Sprintf("Assignees: %s", strings.Join(assignStringList, ", "))
	}
	operator = fmt.Sprintf("Operator: %s", nameFormatter(p.Sender.UserName))
	return issueTitle, link, by, operator, operateResult, assignees
}

// getIssuesCommentInfo gets the information for a comment
func getIssuesCommentInfo(p *api.IssueCommentPayload, nameFormatter nameFormatter) (title, link, by, operator string) {
	title = fmt.Sprintf("[Comment-%s #%d]: %s\n%s", p.Repository.FullName, p.Issue.Index, p.Action, p.Issue.Title)
	link = p.Issue.HTMLURL
	if p.IsPull {
		by = fmt.Sprintf("PullRequest by %s", nameFormatter(p.Issue.Poster.UserName))
	} else {
		by = fmt.Sprintf("Issue by %s", nameFormatter(p.Issue.Poster.UserName))
	}
	operator = fmt.Sprintf("Operator: %s", nameFormatter(p.Sender.UserName))
	return title, link, by, operator
}

type webhookPayloadFormatter struct {
	linkFormatter            linkFormatter
	nameFormatter            nameFormatter
	withSender, withRepoName bool
}

func (wpf webhookPayloadFormatter) getIssuesPayloadInfo(p *api.IssuePayload) (text, issueTitle, attachmentText string, color int) {
	issueTitle = fmt.Sprintf("#%d %s", p.Index, p.Issue.Title)
	titleLink := wpf.linkFormatter(fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Index), issueTitle)
	color = yellowColor

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("Issue opened: %s", titleLink)
		color = orangeColor
	case api.HookIssueClosed:
		text = fmt.Sprintf("Issue closed: %s", titleLink)
		color = redColor
	case api.HookIssueReOpened:
		text = fmt.Sprintf("Issue re-opened: %s", titleLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("Issue edited: %s", titleLink)
	case api.HookIssueAssigned:
		list := make([]string, len(p.Issue.Assignees))
		for i, user := range p.Issue.Assignees {
			list[i] = wpf.linkFormatter(setting.AppURL+url.PathEscape(user.UserName), user.UserName)
		}
		text = fmt.Sprintf("Issue assigned to %s: %s", strings.Join(list, ", "), titleLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("Issue unassigned: %s", titleLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("Issue labels updated: %s", titleLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("Issue labels cleared: %s", titleLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("Issue synchronized: %s", titleLink)
	case api.HookIssueMilestoned:
		text = fmt.Sprintf("Issue milestoned to %s: %s", p.Issue.Milestone.Title, titleLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("Issue milestone cleared: %s", titleLink)
	}

	if wpf.withRepoName {
		text = fmt.Sprintf("[%s] %s", p.Repository.FullName, text)
	}
	if wpf.withSender {
		text += fmt.Sprintf(" by %s", wpf.nameFormatter(p.Sender.UserName))
	}

	if p.Action == api.HookIssueOpened || p.Action == api.HookIssueEdited {
		attachmentText = p.Issue.Body
	}

	return text, issueTitle, attachmentText, color
}

func (wpf webhookPayloadFormatter) getPullRequestPayloadInfo(p *api.PullRequestPayload) (text, issueTitle, attachmentText string, color int) {
	issueTitle = fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := wpf.linkFormatter(p.PullRequest.HTMLURL, issueTitle)
	color = yellowColor

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("Pull request opened: %s", titleLink)
		attachmentText = p.PullRequest.Body
		color = greenColor
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			text = fmt.Sprintf("Pull request merged: %s", titleLink)
			color = purpleColor
		} else {
			text = fmt.Sprintf("Pull request closed: %s", titleLink)
			color = redColor
		}
	case api.HookIssueReOpened:
		text = fmt.Sprintf("Pull request re-opened: %s", titleLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("Pull request edited: %s", titleLink)
		attachmentText = p.PullRequest.Body
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = wpf.linkFormatter(setting.AppURL+user.UserName, user.UserName)
		}
		text = fmt.Sprintf("Pull request assigned to %s: %s", strings.Join(list, ", "), titleLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("Pull request unassigned: %s", titleLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("Pull request labels updated: %s", titleLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("Pull request labels cleared: %s", titleLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("Pull request synchronized: %s", titleLink)
	case api.HookIssueMilestoned:
		text = fmt.Sprintf("Pull request milestoned to %s: %s", p.PullRequest.Milestone.Title, titleLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("Pull request milestone cleared: %s", titleLink)
	case api.HookIssueReviewed:
		text = fmt.Sprintf("Pull request reviewed: %s", titleLink)
		attachmentText = p.Review.Content
	case api.HookIssueReviewRequested:
		text = fmt.Sprintf("Pull request review requested: %s", titleLink)
	case api.HookIssueReviewRequestRemoved:
		text = fmt.Sprintf("Pull request review request removed: %s", titleLink)
	}
	if wpf.withRepoName {
		text = fmt.Sprintf("[%s] %s", p.Repository.FullName, text)
	}
	if wpf.withSender {
		text += fmt.Sprintf(" by %s", wpf.nameFormatter(p.Sender.UserName))
	}

	return text, issueTitle, attachmentText, color
}

func (wpf webhookPayloadFormatter) getReleasePayloadInfo(p *api.ReleasePayload) (text string, color int) {
	refLink := wpf.linkFormatter(p.Repository.HTMLURL+"/releases/tag/"+util.PathEscapeSegments(p.Release.TagName), p.Release.TagName)

	switch p.Action {
	case api.HookReleasePublished:
		text = fmt.Sprintf("Release created: %s", refLink)
		color = greenColor
	case api.HookReleaseUpdated:
		text = fmt.Sprintf("Release updated: %s", refLink)
		color = yellowColor
	case api.HookReleaseDeleted:
		text = fmt.Sprintf("Release deleted: %s", refLink)
		color = redColor
	}
	if wpf.withRepoName {
		text = fmt.Sprintf("[%s] %s", p.Repository.FullName, text)
	}
	if wpf.withSender {
		text += fmt.Sprintf(" by %s", wpf.nameFormatter(p.Sender.UserName))
	}

	return text, color
}

func (wpf webhookPayloadFormatter) getWikiPayloadInfo(p *api.WikiPayload, withCommitMessage bool) (text string, color int, pageLink string) {
	pageLink = wpf.linkFormatter(p.Repository.HTMLURL+"/wiki/"+url.PathEscape(p.Page), p.Page)

	color = greenColor

	switch p.Action {
	case api.HookWikiCreated:
		text = fmt.Sprintf("New wiki page \"%s\"", pageLink)
	case api.HookWikiEdited:
		text = fmt.Sprintf("Wiki page \"%s\" edited", pageLink)
		color = yellowColor
	case api.HookWikiDeleted:
		text = fmt.Sprintf("Wiki page \"%s\" deleted", pageLink)
		color = redColor
	}

	if p.Action != api.HookWikiDeleted && p.Comment != "" && withCommitMessage {
		text += fmt.Sprintf(" (%s)", p.Comment)
	}

	if wpf.withRepoName {
		text = fmt.Sprintf("[%s] %s", p.Repository.FullName, text)
	}
	if wpf.withSender {
		text += fmt.Sprintf(" by %s", wpf.nameFormatter(p.Sender.UserName))
	}

	return text, color, pageLink
}

func (wpf webhookPayloadFormatter) getIssueCommentPayloadInfo(p *api.IssueCommentPayload) (text, issueTitle string, color int) {
	issueTitle = fmt.Sprintf("#%d %s", p.Issue.Index, p.Issue.Title)

	var typ, titleLink string
	color = yellowColor

	if p.IsPull {
		typ = "pull request"
		titleLink = wpf.linkFormatter(p.Comment.PRURL, issueTitle)
	} else {
		typ = "issue"
		titleLink = wpf.linkFormatter(p.Comment.IssueURL, issueTitle)
	}

	switch p.Action {
	case api.HookIssueCommentCreated:
		text = fmt.Sprintf("New comment on %s %s", typ, titleLink)
		if p.IsPull {
			color = greenColorLight
		} else {
			color = orangeColorLight
		}
	case api.HookIssueCommentEdited:
		text = fmt.Sprintf("Comment edited on %s %s", typ, titleLink)
	case api.HookIssueCommentDeleted:
		text = fmt.Sprintf("Comment deleted on %s %s", typ, titleLink)
		color = redColor
	}
	if wpf.withRepoName {
		text = fmt.Sprintf("[%s] %s", p.Repository.FullName, text)
	}
	if wpf.withSender {
		text += fmt.Sprintf(" by %s", wpf.nameFormatter(p.Sender.UserName))
	}

	return text, issueTitle, color
}

func (wpf webhookPayloadFormatter) getPackagePayloadInfo(p *api.PackagePayload) (text string, color int) {
	refLink := wpf.linkFormatter(p.Package.HTMLURL, p.Package.Name+":"+p.Package.Version)

	switch p.Action {
	case api.HookPackageCreated:
		text = fmt.Sprintf("Package created: %s", refLink)
		color = greenColor
	case api.HookPackageDeleted:
		text = fmt.Sprintf("Package deleted: %s", refLink)
		color = redColor
	}
	if wpf.withSender {
		text += fmt.Sprintf(" by %s", wpf.nameFormatter(p.Sender.UserName))
	}

	return text, color
}

func (wpf webhookPayloadFormatter) getActionPayloadInfo(p *api.ActionPayload) (text string, color int) {
	runLink := wpf.linkFormatter(p.Run.HTMLURL, p.Run.Title)
	repoLink := wpf.linkFormatter(p.Run.Repo.HTMLURL, p.Run.Repo.FullName)

	switch p.Action {
	case api.HookActionFailure:
		text = fmt.Sprintf("%s Action Failed in %s %s", runLink, repoLink, p.Run.PrettyRef)
		color = redColor
	case api.HookActionRecover:
		text = fmt.Sprintf("%s Action Recovered in %s %s", runLink, repoLink, p.Run.PrettyRef)
		color = greenColor
	case api.HookActionSuccess:
		text = fmt.Sprintf("%s Action Succeeded in %s %s", runLink, repoLink, p.Run.PrettyRef)
		color = greenColor
	}

	return text, color
}

// ToHook convert models.Webhook to api.Hook
// This function is not part of the convert package to prevent an import cycle
func ToHook(repoLink string, w *webhook_model.Webhook) (*api.Hook, error) {
	// config is deprecated, but kept for compatibility
	config := map[string]string{
		"url":          w.URL,
		"content_type": w.ContentType.Name(),
	}
	if w.Type == webhook_module.SLACK {
		if s, ok := (slackHandler{}.Metadata(w)).(*SlackMeta); ok {
			config["channel"] = s.Channel
			config["username"] = s.Username
			config["icon_url"] = s.IconURL
			config["color"] = s.Color
		}
	}

	authorizationHeader, err := w.HeaderAuthorization()
	if err != nil {
		return nil, err
	}
	var metadata any
	if handler := GetWebhookHandler(w.Type); handler != nil {
		metadata = handler.Metadata(w)
	}

	return &api.Hook{
		ID:                  w.ID,
		Type:                w.Type,
		BranchFilter:        w.BranchFilter,
		URL:                 w.URL,
		Config:              config,
		Events:              w.EventsArray(),
		AuthorizationHeader: authorizationHeader,
		ContentType:         w.ContentType.Name(),
		Metadata:            metadata,
		Active:              w.IsActive,
		Updated:             w.UpdatedUnix.AsTime(),
		Created:             w.CreatedUnix.AsTime(),
	}, nil
}
