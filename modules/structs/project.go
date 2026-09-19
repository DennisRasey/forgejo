// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package structs

type CreateOrUpdateProjectOptions struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	TemplateType string `json:"template_type"`
	CardType     string `json:"card_type"`
	Status       string `json:"status"`
}

type ProjectIssue struct {
	IssueID int64 `json:"issueID"`
	Sorting int64 `json:"sorting"`
}

type MovedIssuesOption struct {
	ProjectIssues []ProjectIssue `json:"issues"`
}

func (m *MovedIssuesOption) GetIssueIDs() []int64 {
	issueIDs := make([]int64, 0, len(m.ProjectIssues))
	for _, issue := range m.ProjectIssues {
		issueIDs = append(issueIDs, issue.IssueID)
	}
	return issueIDs
}

func (m *MovedIssuesOption) GetSortingsMap() map[int64]int64 {
	sortedIssueIDs := make(map[int64]int64)
	for _, issue := range m.ProjectIssues {
		sortedIssueIDs[issue.Sorting] = issue.IssueID
	}
	return sortedIssueIDs
}
