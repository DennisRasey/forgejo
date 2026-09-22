// Copyright 2019 The Gitea Authors. All rights reserved.
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

func TestTelegramPayload(t *testing.T) {
	tc := telegramConvertor{}

	t.Run("Correct webhook params", func(t *testing.T) {
		p := createTelegramPayload("testMsg ")

		assert.Equal(t, "HTML", p.ParseMode)
		assert.True(t, p.DisableWebPreview)
		assert.Equal(t, "testMsg", p.Message)
	})

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := tc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] branch <a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a> created`, pl.Message)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := tc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] branch <a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a> deleted`, pl.Message)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := tc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, `test/repo2 is forked to <a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>`, pl.Message)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := tc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo:test] 2 new commits
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1`, pl.Message)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := tc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Issue opened: <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by user1

issue body`, pl.Message)

		p.Action = api.HookIssueClosed
		pl, err = tc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Issue closed: <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by user1`, pl.Message)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := tc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] New comment on issue <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by user1
more info needed`, pl.Message)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := tc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Pull request opened: <a href="http://localhost:3000/test/repo/pulls/12" rel="nofollow">#12 Fix bug</a> by user1
fixes bug #2`, pl.Message)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := tc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] New comment on pull request <a href="http://localhost:3000/test/repo/pulls/12" rel="nofollow">#12 Fix bug</a> by user1
changes requested`, pl.Message)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := tc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Pull request review approved: #12 Fix bug
good job`, pl.Message)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := tc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Repository created`, pl.Message)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := tc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, `Package created: <a href="http://localhost:3000/user1/-/packages/container/GiteaContainer/latest" rel="nofollow">GiteaContainer:latest</a> by user1`, pl.Message)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := tc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] New wiki page &#34;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#34; (Wiki change comment) by user1`, pl.Message)

		p.Action = api.HookWikiEdited
		pl, err = tc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Wiki page &#34;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#34; edited (Wiki change comment) by user1`, pl.Message)

		p.Action = api.HookWikiDeleted
		pl, err = tc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Wiki page &#34;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#34; deleted by user1`, pl.Message)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := tc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Release created: <a href="http://localhost:3000/test/repo/releases/tag/v1.0" rel="nofollow">v1.0</a> by user1`, pl.Message)
	})

	t.Run("WorkflowRun", func(t *testing.T) {
		testCases := []struct {
			runStatus    actions_model.Status
			expectedText string
		}{
			{
				runStatus: actions_model.StatusBlocked,
				expectedText: `[acme/test] Workflow run &#34;Update README.md&#34; is blocked

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusCancelled,
				expectedText: `[acme/test] Workflow run &#34;Update README.md&#34; was cancelled

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusFailure,
				expectedText: `[acme/test] Workflow run &#34;Update README.md&#34; has failed

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusRunning,
				expectedText: `[acme/test] Workflow run &#34;Update README.md&#34; has started running

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusSkipped,
				expectedText: `[acme/test] Workflow run &#34;Update README.md&#34; was skipped

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusSuccess,
				expectedText: `[acme/test] Workflow run &#34;Update README.md&#34; has completed successfully

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus: actions_model.StatusWaiting,
				expectedText: `[acme/test] Workflow run &#34;Update README.md&#34; is waiting

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

				payload, err := tc.WorkflowRun(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedText, payload.Message)
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
				expectedText: `[acme/test] Workflow job &#34;build-and-test&#34; is blocked

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusCancelled,
				expectedText: `[acme/test] Workflow job &#34;build-and-test&#34; was cancelled

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusFailure,
				expectedText: `[acme/test] Workflow job &#34;build-and-test&#34; has failed

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusRunning,
				expectedText: `[acme/test] Workflow job &#34;build-and-test&#34; has started running

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusSkipped,
				expectedText: `[acme/test] Workflow job &#34;build-and-test&#34; was skipped

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusSuccess,
				expectedText: `[acme/test] Workflow job &#34;build-and-test&#34; has completed successfully

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus: actions_model.StatusWaiting,
				expectedText: `[acme/test] Workflow job &#34;build-and-test&#34; is waiting

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

				payload, err := tc.WorkflowJob(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedText, payload.Message)
			})
		}
	})
}

func TestTelegramJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.TELEGRAM,
		URL:        "https://telegram.example.com/",
		Meta:       ``,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := telegramHandler{}.NewRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://telegram.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body TelegramPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, `[test/repo:test] 2 new commits
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1`, body.Message)
}
