// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

export function getIssueIcon(issue) {
  if (issue.is_pr) {
    if (issue.state === 'open') {
      if (issue.draft === true) {
        return 'octicon-git-pull-request-draft'; // WIP PR
      }
      return 'octicon-git-pull-request'; // Open PR
    } else if (issue.merged === true) {
      return 'octicon-git-merge'; // Merged PR
    }
    return 'octicon-git-pull-request'; // Closed PR
  } else if (issue.state === 'open') {
    return 'octicon-issue-opened'; // Open Issue
  }
  return 'octicon-issue-closed'; // Closed Issue
}

export function getIssueColor(issue) {
  if (issue.is_pr) {
    if (issue.draft === true) {
      return 'grey'; // WIP PR
    } else if (issue.merged === true) {
      return 'purple'; // Merged PR
    }
  }
  if (issue.state === 'open') {
    return 'green'; // Open Issue
  }
  return 'red'; // Closed Issue
}
