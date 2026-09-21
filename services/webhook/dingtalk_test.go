// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"net/url"
	"testing"

	actions_model "forgejo.org/models/actions"
	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/json"
	api "forgejo.org/modules/structs"
	webhook_module "forgejo.org/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDingTalkPayload(t *testing.T) {
	parseRealSingleURL := func(singleURL string) string {
		if u, err := url.Parse(singleURL); err == nil {
			assert.Equal(t, "dingtalk", u.Scheme)
			assert.Equal(t, "dingtalkclient", u.Host)
			assert.Equal(t, "/page/link", u.Path)
			return u.Query().Get("url")
		}
		return ""
	}
	dc := dingtalkConvertor{}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := dc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] branch test created", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] branch test created", pl.ActionCard.Title)
		assert.Equal(t, "view ref test", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := dc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] branch test deleted", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] branch test deleted", pl.ActionCard.Title)
		assert.Equal(t, "view ref test", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := dc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, "test/repo2 is forked to test/repo", pl.ActionCard.Text)
		assert.Equal(t, "test/repo2 is forked to test/repo", pl.ActionCard.Title)
		assert.Equal(t, "view forked repo test/repo", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.ActionCard.Title)
		assert.Equal(t, "view commits", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := dc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue opened: #2 crash by user1\r\n\r\nissue body", pl.ActionCard.Text)
		assert.Equal(t, "#2 crash", pl.ActionCard.Title)
		assert.Equal(t, "view issue", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", parseRealSingleURL(pl.ActionCard.SingleURL))

		p.Action = api.HookIssueClosed
		pl, err = dc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue closed: #2 crash by user1", pl.ActionCard.Text)
		assert.Equal(t, "#2 crash", pl.ActionCard.Title)
		assert.Equal(t, "view issue", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on issue #2 crash by user1\r\n\r\nmore info needed", pl.ActionCard.Text)
		assert.Equal(t, "#2 crash", pl.ActionCard.Title)
		assert.Equal(t, "view issue comment", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := dc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug by user1\r\n\r\nfixes bug #2", pl.ActionCard.Text)
		assert.Equal(t, "#12 Fix bug", pl.ActionCard.Title)
		assert.Equal(t, "view pull request", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug by user1\r\n\r\nchanges requested", pl.ActionCard.Text)
		assert.Equal(t, "#12 Fix bug", pl.ActionCard.Title)
		assert.Equal(t, "view issue comment", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := dc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug", pl.ActionCard.Title)
		assert.Equal(t, "view pull request", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := dc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Repository created", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] Repository created", pl.ActionCard.Title)
		assert.Equal(t, "view repository", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := dc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, "Package created: GiteaContainer:latest by user1", pl.ActionCard.Text)
		assert.Equal(t, "Package created: GiteaContainer:latest by user1", pl.ActionCard.Title)
		assert.Equal(t, "view package", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := dc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New wiki page \"index\" (Wiki change comment) by user1", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] New wiki page \"index\" (Wiki change comment) by user1", pl.ActionCard.Title)
		assert.Equal(t, "view wiki", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", parseRealSingleURL(pl.ActionCard.SingleURL))

		p.Action = api.HookWikiEdited
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page \"index\" edited (Wiki change comment) by user1", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] Wiki page \"index\" edited (Wiki change comment) by user1", pl.ActionCard.Title)
		assert.Equal(t, "view wiki", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", parseRealSingleURL(pl.ActionCard.SingleURL))

		p.Action = api.HookWikiDeleted
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page \"index\" deleted by user1", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] Wiki page \"index\" deleted by user1", pl.ActionCard.Title)
		assert.Equal(t, "view wiki", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := dc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.ActionCard.Text)
		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.ActionCard.Title)
		assert.Equal(t, "view release", pl.ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/releases/tag/v1.0", parseRealSingleURL(pl.ActionCard.SingleURL))
	})

	t.Run("WorkflowRun", func(t *testing.T) {
		testCases := []struct {
			runStatus     actions_model.Status
			expectedTitle string
			expectedText  string
		}{
			{
				runStatus:     actions_model.StatusBlocked,
				expectedTitle: `[acme/test] Workflow run "Update README.md" is blocked`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus:     actions_model.StatusCancelled,
				expectedTitle: `[acme/test] Workflow run "Update README.md" was cancelled`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus:     actions_model.StatusFailure,
				expectedTitle: `[acme/test] Workflow run "Update README.md" has failed`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus:     actions_model.StatusRunning,
				expectedTitle: `[acme/test] Workflow run "Update README.md" has started running`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus:     actions_model.StatusSkipped,
				expectedTitle: `[acme/test] Workflow run "Update README.md" was skipped`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus:     actions_model.StatusSuccess,
				expectedTitle: `[acme/test] Workflow run "Update README.md" has completed successfully`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.`,
			},
			{
				runStatus:     actions_model.StatusWaiting,
				expectedTitle: `[acme/test] Workflow run "Update README.md" is waiting`,
				expectedText: `Repository: acme/test
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
					},
				}

				payload, err := dc.WorkflowRun(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedTitle, payload.ActionCard.Title)
				assert.Equal(t, testCase.expectedText, payload.ActionCard.Text)
				assert.Equal(t, "view run", payload.ActionCard.SingleTitle)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/197719",
					parseRealSingleURL(payload.ActionCard.SingleURL))
			})
		}
	})

	t.Run("WorkflowJob", func(t *testing.T) {
		testCases := []struct {
			jobStatus     actions_model.Status
			expectedTitle string
			expectedText  string
		}{
			{
				jobStatus:     actions_model.StatusBlocked,
				expectedTitle: `[acme/test] Workflow job "build-and-test" is blocked`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus:     actions_model.StatusCancelled,
				expectedTitle: `[acme/test] Workflow job "build-and-test" was cancelled`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus:     actions_model.StatusFailure,
				expectedTitle: `[acme/test] Workflow job "build-and-test" has failed`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus:     actions_model.StatusRunning,
				expectedTitle: `[acme/test] Workflow job "build-and-test" has started running`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus:     actions_model.StatusSkipped,
				expectedTitle: `[acme/test] Workflow job "build-and-test" was skipped`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus:     actions_model.StatusSuccess,
				expectedTitle: `[acme/test] Workflow job "build-and-test" has completed successfully`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.`,
			},
			{
				jobStatus:     actions_model.StatusWaiting,
				expectedTitle: `[acme/test] Workflow job "build-and-test" is waiting`,
				expectedText: `Repository: acme/test
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
					},
					Repository: &api.Repository{
						FullName: "acme/test",
					},
				}

				payload, err := dc.WorkflowJob(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedTitle, payload.ActionCard.Title)
				assert.Equal(t, testCase.expectedText, payload.ActionCard.Text)
				assert.Equal(t, "view job", payload.ActionCard.SingleTitle)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1",
					parseRealSingleURL(payload.ActionCard.SingleURL))
			})
		}
	})
}

func TestDingTalkJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.DINGTALK,
		URL:        "https://dingtalk.example.com/",
		Meta:       ``,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := dingtalkHandler{}.NewRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://dingtalk.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body DingtalkPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", body.ActionCard.Text)
}
