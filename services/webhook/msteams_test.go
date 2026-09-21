// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"bytes"
	"context"
	"strings"
	"testing"

	actions_model "forgejo.org/models/actions"
	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/json"
	api "forgejo.org/modules/structs"
	webhook_module "forgejo.org/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findTextInContainer recursively searches for text within an MSTeamsContainer
func findTextInContainer(c MSTeamsContainer, substr string) bool {
	for _, it := range c.Items {
		switch v := it.(type) {
		case MSTeamsTextBlock:
			if strings.Contains(v.Text, substr) {
				return true
			}
		case MSTeamsColumnSet:
			for _, col := range v.Columns {
				for _, it2 := range col.Items {
					if tb, ok := it2.(MSTeamsTextBlock); ok && strings.Contains(tb.Text, substr) {
						return true
					}
				}
			}
		case MSTeamsContainer:
			if findTextInContainer(v, substr) {
				return true
			}
		case MSTeamsFactSet:
			for _, fact := range v.Facts {
				if strings.Contains(fact.Value, substr) || strings.Contains(fact.Title, substr) {
					return true
				}
			}
		}
	}
	return false
}

func TestMSTeamsPayload(t *testing.T) {
	mc := msteamsConvertor{}

	// helper to find text within the adaptive card body
	findTextInBody := func(pl MSTeamsPayload, substr string) bool {
		for _, container := range pl.Body {
			if findTextInContainer(container, substr) {
				return true
			}
		}
		return false
	}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := mc.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check payload structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.Equal(t, "1.5", pl.Version)

		// Check body structure: header + title + badge sections
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Header should contain repo info
		assert.True(t, findTextInBody(pl, "test/repo"))

		// Title should contain action by user
		assert.True(t, findTextInBody(pl, "Branch created: test"))

		// action button should point to branch
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Actions[0].URL)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := mc.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "test/repo"))
		assert.True(t, findTextInBody(pl, "Branch deleted: test"))

		// action button should point to branch
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.Actions[0].URL)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := mc.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "[test/repo2](http://localhost:3000/test/repo2) is forked to test/repo"))

		// action button should point to repo
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.Actions[0].URL)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := mc.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify repo and basic content
		assert.True(t, findTextInBody(pl, "[test] 2 new commits"))

		// commit details should be present in body
		assert.True(t, findTextInBody(pl, "2020558"))
		assert.True(t, findTextInBody(pl, "commit message"))

		// action button should point to compare
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Actions[0].URL)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := mc.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "test/repo"))
		assert.True(t, findTextInBody(pl, "Issue opened: #2 crash"))
		assert.True(t, findTextInBody(pl, "issue body"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.Actions[0].URL)

		p.Action = api.HookIssueClosed
		pl, err = mc.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		assert.True(t, findTextInBody(pl, "Issue closed: #2 crash"))
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.Actions[0].URL)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "test/repo"))
		assert.True(t, findTextInBody(pl, "New comment on issue #2 crash"))
		assert.True(t, findTextInBody(pl, "more info needed"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.Actions[0].URL)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.PullRequest.Head = &api.PRBranchInfo{
			Name:   "feature/test",
			Ref:    "feature/test",
			Sha:    "b1eb92dc659513b7b4eb57d7ee7f9c6f92e714b5",
			RepoID: 1,
			Repository: &api.Repository{
				HTMLURL:  "http://localhost:3000/test/repo",
				Name:     "repo",
				FullName: "test/repo",
			},
		}

		pl, err := mc.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "test/repo"))
		assert.True(t, findTextInBody(pl, "Pull request opened: #12 Fix bug"))
		assert.True(t, findTextInBody(pl, "fixes bug #2"))
		assert.True(t, findTextInBody(pl, "feature/test → refs/pull/2/head"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.Actions[0].URL)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "test/repo"))
		assert.True(t, findTextInBody(pl, "New comment on pull request #12 Fix bug"))
		assert.True(t, findTextInBody(pl, "changes requested"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", pl.Actions[0].URL)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := mc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// review content should be present
		assert.True(t, findTextInBody(pl, "Pull request review approved: #12 Fix bug"))
		assert.True(t, findTextInBody(pl, "good job"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.Actions[0].URL)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := mc.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "Repository created: test/repo"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.Actions[0].URL)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := mc.Package(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// no repo is associated
		assert.False(t, findTextInBody(pl, "test/repo"))
		// Verify content
		assert.True(t, findTextInBody(pl, "Package created: GiteaContainer:latest"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest", pl.Actions[0].URL)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := mc.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content for create
		assert.True(t, findTextInBody(pl, "New wiki page \"index\""))
		assert.True(t, findTextInBody(pl, "Wiki change comment"))
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Actions[0].URL)

		p.Action = api.HookWikiEdited
		pl, err = mc.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		assert.True(t, findTextInBody(pl, "Wiki page \"index\" edited"))
		assert.True(t, findTextInBody(pl, "Wiki change comment"))
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Actions[0].URL)

		p.Action = api.HookWikiDeleted
		pl, err = mc.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		assert.True(t, findTextInBody(pl, "Wiki page \"index\" deleted"))
		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Actions[0].URL)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := mc.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		// Check basic structure
		require.Equal(t, "AdaptiveCard", pl.Type)
		require.GreaterOrEqual(t, len(pl.Body), 2)

		// Verify content
		assert.True(t, findTextInBody(pl, "Release created: v1.0"))

		require.Len(t, pl.Actions, 1)
		assert.Equal(t, "View in Forgejo", pl.Actions[0].Title)
		assert.Equal(t, "http://localhost:3000/test/repo/releases/tag/v1.0", pl.Actions[0].URL)
	})

	t.Run("WorkflowRun", func(t *testing.T) {
		testCases := []struct {
			runStatus    actions_model.Status
			expectedText string
		}{
			{
				runStatus:    actions_model.StatusBlocked,
				expectedText: `Workflow run "Update README.md" is blocked, triggered by @jane`,
			},
			{
				runStatus:    actions_model.StatusCancelled,
				expectedText: `Workflow run "Update README.md" was cancelled, triggered by @jane`,
			},
			{
				runStatus:    actions_model.StatusFailure,
				expectedText: `Workflow run "Update README.md" has failed, triggered by @jane`,
			},
			{
				runStatus:    actions_model.StatusRunning,
				expectedText: `Workflow run "Update README.md" has started running, triggered by @jane`,
			},
			{
				runStatus:    actions_model.StatusSkipped,
				expectedText: `Workflow run "Update README.md" was skipped, triggered by @jane`,
			},
			{
				runStatus:    actions_model.StatusSuccess,
				expectedText: `Workflow run "Update README.md" has completed successfully, triggered by @jane`,
			},
			{
				runStatus:    actions_model.StatusWaiting,
				expectedText: `Workflow run "Update README.md" is waiting, triggered by @jane`,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.runStatus.String(), func(t *testing.T) {
				inputPayload := &api.WorkflowRunPayload{
					Action: api.HookNewWorkflowRunAttempt,
					Run: &api.ActionRun{
						Title:   "Update README.md",
						Status:  testCase.runStatus.String(),
						HTMLURL: "https://example.com/acme/test/actions/runs/197719",
						Repo: &api.Repository{
							FullName: "acme/test",
							HTMLURL:  "https://example.com/acme/test",
						},
						TriggerUser: &api.User{
							UserName:  "jane",
							AvatarURL: "https://example.com/avatars/7dc9cf?size=64",
						},
					},
				}

				payload, err := mc.WorkflowRun(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, "AdaptiveCard", payload.Type)
				assert.Len(t, payload.Body, 3)

				assert.True(t, findTextInBody(payload, "💬 Update | [acme/test](https://example.com/acme/test)"))
				assert.True(t, findTextInBody(payload, testCase.expectedText))
				assert.True(t, findTextInBody(payload, "Repository: acme/test"))
				assert.True(t, findTextInBody(payload, "Run: Update README.md"))

				assert.Len(t, payload.Actions, 1)
				assert.Equal(t, "View in Forgejo", payload.Actions[0].Title)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/197719",
					payload.Actions[0].URL)
			})
		}
	})

	t.Run("WorkflowJob", func(t *testing.T) {
		testCases := []struct {
			jobStatus    actions_model.Status
			expectedText string
		}{
			{
				jobStatus:    actions_model.StatusBlocked,
				expectedText: `Workflow job "build-and-test" is blocked, triggered by @jane`,
			},
			{
				jobStatus:    actions_model.StatusCancelled,
				expectedText: `Workflow job "build-and-test" was cancelled, triggered by @jane`,
			},
			{
				jobStatus:    actions_model.StatusFailure,
				expectedText: `Workflow job "build-and-test" has failed, triggered by @jane`,
			},
			{
				jobStatus:    actions_model.StatusRunning,
				expectedText: `Workflow job "build-and-test" has started running, triggered by @jane`,
			},
			{
				jobStatus:    actions_model.StatusSkipped,
				expectedText: `Workflow job "build-and-test" was skipped, triggered by @jane`,
			},
			{
				jobStatus:    actions_model.StatusSuccess,
				expectedText: `Workflow job "build-and-test" has completed successfully, triggered by @jane`,
			},
			{
				jobStatus:    actions_model.StatusWaiting,
				expectedText: `Workflow job "build-and-test" is waiting, triggered by @jane`,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.jobStatus.String(), func(t *testing.T) {
				inputPayload := &api.WorkflowJobPayload{
					Action: api.HookNewWorkflowJobAttempt,
					Job: &api.ActionRunJob{
						Name:    "build-and-test",
						HTMLURL: "https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1",
						Status:  testCase.jobStatus.String(),
					},
					Run: &api.ActionRun{
						Title: "Update README.md",
						TriggerUser: &api.User{
							UserName:  "jane",
							AvatarURL: "https://example.com/avatars/7dc9cf?size=64",
						},
					},
					Repository: &api.Repository{
						FullName: "acme/test",
						HTMLURL:  "https://example.com/acme/test",
					},
				}

				payload, err := mc.WorkflowJob(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, "AdaptiveCard", payload.Type)
				assert.Len(t, payload.Body, 3)

				assert.True(t, findTextInBody(payload, "💬 Update | [acme/test](https://example.com/acme/test)"))
				assert.True(t, findTextInBody(payload, testCase.expectedText))
				assert.True(t, findTextInBody(payload, "Repository: acme/test"))
				assert.True(t, findTextInBody(payload, "Run: Update README.md"))
				assert.True(t, findTextInBody(payload, "Job: build-and-test"))

				assert.Len(t, payload.Actions, 1)
				assert.Equal(t, "View in Forgejo", payload.Actions[0].Title)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1",
					payload.Actions[0].URL)
			})
		}
	})
}

func TestMSTeamsJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)
	require.NotNil(t, data)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.MSTEAMS,
		URL:        "https://msteams.example.com/",
		Meta:       ``,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := msteamsHandler{}.NewRequest(context.Background(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://msteams.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body MSTeamsPayload
	err = json.NewDecoder(bytes.NewReader(reqBody)).Decode(&body)
	require.NoError(t, err)

	// Verify payload structure
	assert.Equal(t, "AdaptiveCard", body.Type)
	assert.Equal(t, "1.5", body.Version)
}
