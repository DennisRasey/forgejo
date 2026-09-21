// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"testing"

	actions_model "forgejo.org/models/actions"
	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/json"
	api "forgejo.org/modules/structs"
	webhook_module "forgejo.org/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackPayload(t *testing.T) {
	sc := slackConvertor{}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := sc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo:<http://localhost:3000/test/repo/src/branch/test|test>] branch created by `user1`", pl.Text)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := sc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>:test] branch deleted by `user1`", pl.Text)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := sc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, "<http://localhost:3000/test/repo2|test/repo2> is forked to <http://localhost:3000/test/repo|test/repo>", pl.Text)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := sc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo:<http://localhost:3000/test/repo/src/branch/test|test>] 2 new commits pushed by `user1`", pl.Text)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := sc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue opened: <http://localhost:3000/test/repo/issues/2|#2 crash> by `user1`", pl.Text)

		p.Action = api.HookIssueClosed
		pl, err = sc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue closed: <http://localhost:3000/test/repo/issues/2|#2 crash> by `user1`", pl.Text)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := sc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on issue <http://localhost:3000/test/repo/issues/2|#2 crash> by `user1`", pl.Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := sc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request opened: <http://localhost:3000/test/repo/pulls/12|#12 Fix bug> by `user1`", pl.Text)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := sc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on pull request <http://localhost:3000/test/repo/pulls/12|#12 Fix bug> by `user1`", pl.Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := sc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request review approved: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by `user1`", pl.Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := sc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Repository created by `user1`", pl.Text)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := sc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, "Package created: <http://localhost:3000/user1/-/packages/container/GiteaContainer/latest|GiteaContainer:latest> by `user1`", pl.Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := sc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New wiki page \"<http://localhost:3000/test/repo/wiki/index|index>\" (Wiki change comment) by `user1`", pl.Text)

		p.Action = api.HookWikiEdited
		pl, err = sc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page \"<http://localhost:3000/test/repo/wiki/index|index>\" edited (Wiki change comment) by `user1`", pl.Text)

		p.Action = api.HookWikiDeleted
		pl, err = sc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page \"<http://localhost:3000/test/repo/wiki/index|index>\" deleted by `user1`", pl.Text)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := sc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Release created: <http://localhost:3000/test/repo/releases/tag/v1.0|v1.0> by `user1`", pl.Text)
	})

	t.Run("WorkflowRun", func(t *testing.T) {
		testCases := []struct {
			action                  api.HookWorkflowRunAction
			runStatus               actions_model.Status
			expectedColour          string
			expectedTitle           string
			expectedText            string
			expectedLink            string
			expectedAttachmentTitle string
		}{
			{
				action:         api.HookNewWorkflowRunAttempt,
				runStatus:      actions_model.StatusBlocked,
				expectedColour: fmt.Sprintf("%x", yellowColor),
				expectedTitle:  `[acme/test] Workflow run "Update README.md" is blocked`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
				expectedAttachmentTitle: `New attempt of run "Update README.md" with status blocked`,
			},
			{
				action:         api.HookWorkflowRunCompleted,
				runStatus:      actions_model.StatusCancelled,
				expectedColour: fmt.Sprintf("%x", greyColor),
				expectedTitle:  `[acme/test] Workflow run "Update README.md" was cancelled`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
				expectedAttachmentTitle: `The run "Update README.md" has completed with status cancelled`,
			},
			{
				action:         api.HookWorkflowRunCompleted,
				runStatus:      actions_model.StatusFailure,
				expectedColour: fmt.Sprintf("%x", redColor),
				expectedTitle:  `[acme/test] Workflow run "Update README.md" has failed`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
				expectedAttachmentTitle: `The run "Update README.md" has completed with status failure`,
			},
			{
				action:         api.HookWorkflowRunStatusChanged,
				runStatus:      actions_model.StatusRunning,
				expectedColour: fmt.Sprintf("%x", greenColorLight),
				expectedTitle:  `[acme/test] Workflow run "Update README.md" has started running`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
				expectedAttachmentTitle: `The status of run "Update README.md" is now running`,
			},
			{
				action:         api.HookWorkflowRunCompleted,
				runStatus:      actions_model.StatusSkipped,
				expectedColour: fmt.Sprintf("%x", greyColor),
				expectedTitle:  `[acme/test] Workflow run "Update README.md" was skipped`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
				expectedAttachmentTitle: `The run "Update README.md" has completed with status skipped`,
			},
			{
				action:         api.HookWorkflowRunCompleted,
				runStatus:      actions_model.StatusSuccess,
				expectedColour: fmt.Sprintf("%x", greenColor),
				expectedTitle:  `[acme/test] Workflow run "Update README.md" has completed successfully`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
				expectedAttachmentTitle: `The run "Update README.md" has completed with status success`,
			},
			{
				action:         api.HookNewWorkflowRunAttempt,
				runStatus:      actions_model.StatusWaiting,
				expectedColour: fmt.Sprintf("%x", blueColor),
				expectedTitle:  `[acme/test] Workflow run "Update README.md" is waiting`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
				expectedAttachmentTitle: `New attempt of run "Update README.md" with status waiting`,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.runStatus.String(), func(t *testing.T) {
				inputPayload := &api.WorkflowRunPayload{
					Action: testCase.action,
					Run: &api.ActionRun{
						Title:   "Update README.md",
						HTMLURL: "https://example.com/acme/test/actions/runs/197719",
						Status:  testCase.runStatus.String(),
						Repo: &api.Repository{
							FullName: "acme/test",
						},
						TriggerUser: &api.User{
							UserName:  "jane",
							AvatarURL: "https://example.com/avatars/7dc9cf?size=64",
						},
					},
				}

				payload, err := sc.WorkflowRun(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedTitle, payload.Text)

				assert.Len(t, payload.Attachments, 1)
				assert.Equal(t, testCase.expectedColour, payload.Attachments[0].Color)
				assert.Equal(t, testCase.expectedText, payload.Attachments[0].Text)
				assert.Equal(t, testCase.expectedAttachmentTitle, payload.Attachments[0].Title)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/197719",
					payload.Attachments[0].TitleLink)
			})
		}
	})

	t.Run("WorkflowJob", func(t *testing.T) {
		testCases := []struct {
			action                  api.HookWorkflowJobAction
			jobStatus               actions_model.Status
			expectedColour          string
			expectedTitle           string
			expectedText            string
			expectedLink            string
			expectedAttachmentTitle string
		}{
			{
				action:         api.HookNewWorkflowJobAttempt,
				jobStatus:      actions_model.StatusBlocked,
				expectedColour: fmt.Sprintf("%x", yellowColor),
				expectedTitle:  `[acme/test] Workflow job "build-and-test" is blocked`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
				expectedAttachmentTitle: `New attempt of job "build-and-test" with status blocked`,
			},
			{
				action:         api.HookWorkflowJobCompleted,
				jobStatus:      actions_model.StatusCancelled,
				expectedColour: fmt.Sprintf("%x", greyColor),
				expectedTitle:  `[acme/test] Workflow job "build-and-test" was cancelled`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
				expectedAttachmentTitle: `The job "build-and-test" has completed with status cancelled`,
			},
			{
				action:         api.HookWorkflowJobCompleted,
				jobStatus:      actions_model.StatusFailure,
				expectedColour: fmt.Sprintf("%x", redColor),
				expectedTitle:  `[acme/test] Workflow job "build-and-test" has failed`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
				expectedAttachmentTitle: `The job "build-and-test" has completed with status failure`,
			},
			{
				action:         api.HookWorkflowJobStatusChanged,
				jobStatus:      actions_model.StatusRunning,
				expectedColour: fmt.Sprintf("%x", greenColorLight),
				expectedTitle:  `[acme/test] Workflow job "build-and-test" has started running`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
				expectedAttachmentTitle: `The status of job "build-and-test" is now running`,
			},
			{
				action:         api.HookWorkflowJobCompleted,
				jobStatus:      actions_model.StatusSkipped,
				expectedColour: fmt.Sprintf("%x", greyColor),
				expectedTitle:  `[acme/test] Workflow job "build-and-test" was skipped`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
				expectedAttachmentTitle: `The job "build-and-test" has completed with status skipped`,
			},
			{
				action:         api.HookWorkflowJobCompleted,
				jobStatus:      actions_model.StatusSuccess,
				expectedColour: fmt.Sprintf("%x", greenColor),
				expectedTitle:  `[acme/test] Workflow job "build-and-test" has completed successfully`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
				expectedAttachmentTitle: `The job "build-and-test" has completed with status success`,
			},
			{
				action:         api.HookNewWorkflowJobAttempt,
				jobStatus:      actions_model.StatusWaiting,
				expectedColour: fmt.Sprintf("%x", blueColor),
				expectedTitle:  `[acme/test] Workflow job "build-and-test" is waiting`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
				expectedAttachmentTitle: `New attempt of job "build-and-test" with status waiting`,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.jobStatus.String(), func(t *testing.T) {
				inputPayload := &api.WorkflowJobPayload{
					Action: testCase.action,
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
					},
				}

				payload, err := sc.WorkflowJob(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedTitle, payload.Text)

				assert.Len(t, payload.Attachments, 1)
				assert.Equal(t, testCase.expectedColour, payload.Attachments[0].Color)
				assert.Equal(t, testCase.expectedText, payload.Attachments[0].Text)
				assert.Equal(t, testCase.expectedAttachmentTitle, payload.Attachments[0].Title)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1",
					payload.Attachments[0].TitleLink)
			})
		}
	})
}

func TestSlackJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.SLACK,
		URL:        "https://slack.example.com/",
		Meta:       `{}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := slackHandler{}.NewRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://slack.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body SlackPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "[test/repo:<http://localhost:3000/test/repo/src/branch/test|test>] 2 new commits pushed by `user1`", body.Text)
}

func TestIsValidSlackChannel(t *testing.T) {
	tt := []struct {
		channelName string
		expected    bool
	}{
		{"gitea", true},
		{"#gitea", true},
		{"  ", false},
		{"#", false},
		{" #", false},
		{"gitea   ", false},
		{"  gitea", false},
	}

	for _, v := range tt {
		assert.Equal(t, v.expected, IsValidSlackChannel(v.channelName))
	}
}

func TestSlackMetadata(t *testing.T) {
	w := &webhook_model.Webhook{
		Meta: `{"channel": "foo", "username": "username", "color": "blue"}`,
	}
	slackHook := slackHandler{}.Metadata(w)
	assert.Equal(t, SlackMeta{
		Channel:  "foo",
		Username: "username",
		Color:    "blue",
	},
		*slackHook.(*SlackMeta))
}

func TestSlackToHook(t *testing.T) {
	w := &webhook_model.Webhook{
		Type:        webhook_module.SLACK,
		ContentType: webhook_model.ContentTypeJSON,
		URL:         "https://slack.example.com",
		Meta:        `{"channel": "foo", "username": "username", "color": "blue"}`,
		HookEvent: &webhook_module.HookEvent{
			PushOnly:       true,
			SendEverything: false,
			ChooseEvents:   false,
			HookEvents: webhook_module.HookEvents{
				Create:      false,
				Push:        true,
				PullRequest: false,
			},
		},
	}
	h, err := ToHook("repoLink", w)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"url":          "https://slack.example.com",
		"content_type": "json",

		"channel":  "foo",
		"color":    "blue",
		"icon_url": "",
		"username": "username",
	}, h.Config)
	assert.Equal(t, "https://slack.example.com", h.URL)
	assert.Equal(t, "json", h.ContentType)
	assert.Equal(t, &SlackMeta{
		Channel:  "foo",
		Username: "username",
		IconURL:  "",
		Color:    "blue",
	}, h.Metadata)
}
