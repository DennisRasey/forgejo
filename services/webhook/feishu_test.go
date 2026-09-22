// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/json"
	api "forgejo.org/modules/structs"
	webhook_module "forgejo.org/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeishuPayload(t *testing.T) {
	fc := feishuConvertor{}
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := fc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] branch test created`, pl.Content.Text)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := fc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] branch test deleted`, pl.Content.Text)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := fc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, `test/repo2 is forked to test/repo`, pl.Content.Text)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := fc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo:test] \r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.Content.Text)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := fc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[Issue-test/repo #2]: opened\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.Content.Text)

		p.Action = api.HookIssueClosed
		pl, err = fc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[Issue-test/repo #2]: closed\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.Content.Text)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := fc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[Comment-test/repo #2]: created\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\n\nmore info needed", pl.Content.Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := fc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, "[PullRequest-test/repo #12]: opened\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\nAssignees: user1\n\nfixes bug #2", pl.Content.Text)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := fc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[Comment-test/repo #12]: created\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\n\nchanges requested", pl.Content.Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := fc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", pl.Content.Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := fc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Repository created", pl.Content.Text)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := fc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, "Package created: GiteaContainer:latest by user1", pl.Content.Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New wiki page \"index\" (Wiki change comment) by user1", pl.Content.Text)

		p.Action = api.HookWikiEdited
		pl, err = fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page \"index\" edited (Wiki change comment) by user1", pl.Content.Text)

		p.Action = api.HookWikiDeleted
		pl, err = fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page \"index\" deleted by user1", pl.Content.Text)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := fc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.Content.Text)
	})

	t.Run("WorkflowRun", func(t *testing.T) {
		testCases := []struct {
			runStatus    actions_model.Status
			expectedText string
		}{
			{
				runStatus: actions_model.StatusBlocked,
				expectedText: `[acme/test] Workflow run "Update README.md" is blocked

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusCancelled,
				expectedText: `[acme/test] Workflow run "Update README.md" was cancelled

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusFailure,
				expectedText: `[acme/test] Workflow run "Update README.md" has failed

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusRunning,
				expectedText: `[acme/test] Workflow run "Update README.md" has started running

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusSkipped,
				expectedText: `[acme/test] Workflow run "Update README.md" was skipped

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusSuccess,
				expectedText: `[acme/test] Workflow run "Update README.md" has completed successfully

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusWaiting,
				expectedText: `[acme/test] Workflow run "Update README.md" is waiting

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
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
						},
						TriggerUser: &api.User{
							UserName:  "jane",
							AvatarURL: "https://example.com/avatars/7dc9cf?size=64",
						},
					},
				}

				payload, err := fc.WorkflowRun(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedText, payload.Content.Text)
			})
		}
	})

	t.Run("WorkflowJob", func(t *testing.T) {
		testCases := []struct {
			jobStatus    actions_model.Status
			expectedText string
		}{
			{
				jobStatus: actions_model.StatusBlocked,
				expectedText: `[acme/test] Workflow job "build-and-test" is blocked

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusCancelled,
				expectedText: `[acme/test] Workflow job "build-and-test" was cancelled

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusFailure,
				expectedText: `[acme/test] Workflow job "build-and-test" has failed

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusRunning,
				expectedText: `[acme/test] Workflow job "build-and-test" has started running

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusSkipped,
				expectedText: `[acme/test] Workflow job "build-and-test" was skipped

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusSuccess,
				expectedText: `[acme/test] Workflow job "build-and-test" has completed successfully

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusWaiting,
				expectedText: `[acme/test] Workflow job "build-and-test" is waiting

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
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
						Repo: &api.Repository{
							FullName: "acme/test",
						},
						TriggerUser: &api.User{
							UserName:  "jane",
							AvatarURL: "https://example.com/avatars/7dc9cf?size=64",
						},
					},
				}

				payload, err := fc.WorkflowJob(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedText, payload.Content.Text)
			})
		}
	})
}

func TestFeishuJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.FEISHU,
		URL:        "https://feishu.example.com/",
		Meta:       `{}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := feishuHandler{}.NewRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://feishu.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body FeishuPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "[test/repo:test] \r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", body.Content.Text)
}
