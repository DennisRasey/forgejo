// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"bytes"
	"io"
	"testing"

	actions_model "forgejo.org/models/actions"
	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/json"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	webhook_module "forgejo.org/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscordPayload(t *testing.T) {
	dc := discordConvertor{}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := dc.Create(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] branch test created", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := dc.Delete(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] branch test deleted", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := dc.Fork(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "test/repo2 is forked to test/repo", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Embeds[0].Title)
		assert.Equal(t, "[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message \\- user1\n[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message \\- user1", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PushWithLongCommitMessage", func(t *testing.T) {
		p := pushTestMultilineCommitMessagePayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Embeds[0].Title)
		assert.Equal(t, "[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好... \\- user1\n[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好... \\- user1", pl.Embeds[0].Description)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PushWithMarkdownCharactersInCommitMessage", func(t *testing.T) {
		p := pushTestEscapeCommitMessagePayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Embeds[0].Title)
		assert.Equal(t, "[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) \\# conflicts\n\\# \\- some/conflicting/file.txt \\- user1\n[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) \\# conflicts\n\\# \\- some/conflicting/file.txt \\- user1", pl.Embeds[0].Description)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := dc.Issue(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Issue opened: #2 crash", pl.Embeds[0].Title)
		assert.Equal(t, "issue body", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)

		p.Action = api.HookIssueClosed
		pl, err = dc.Issue(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Issue closed: #2 crash", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)

		j, err := json.Marshal(pl)
		require.NoError(t, err)

		unsetFields := struct {
			Content *string `json:"content"`
			TTS     *bool   `json:"tts"`
			Wait    *bool   `json:"wait"`
			Fields  []any   `json:"fields"`
			Footer  struct {
				Text *string `json:"text"`
			} `json:"footer"`
		}{}

		err = json.Unmarshal(j, &unsetFields)
		require.NoError(t, err)
		assert.Nil(t, unsetFields.Content)
		assert.Nil(t, unsetFields.TTS)
		assert.Nil(t, unsetFields.Wait)
		assert.Nil(t, unsetFields.Fields)
		assert.Nil(t, unsetFields.Footer.Text)
	})

	t.Run("Issue with long title", func(t *testing.T) {
		p := issueTestPayloadWithLongTitle()

		p.Action = api.HookIssueOpened
		pl, err := dc.Issue(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Len(t, pl.Embeds[0].Title, 256)
	})

	t.Run("Issue with long body", func(t *testing.T) {
		p := issueTestPayloadWithLongBody()

		p.Action = api.HookIssueOpened
		pl, err := dc.Issue(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Len(t, pl.Embeds[0].Description, 2000)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] New comment on issue #2 crash", pl.Embeds[0].Title)
		assert.Equal(t, "more info needed", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := dc.PullRequest(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug", pl.Embeds[0].Title)
		assert.Equal(t, "fixes bug #2", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug", pl.Embeds[0].Title)
		assert.Equal(t, "changes requested", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := dc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Pull request review approved: #12 Fix bug", pl.Embeds[0].Title)
		assert.Equal(t, "good job", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := dc.Repository(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Repository created", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := dc.Package(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "Package created: GiteaContainer:latest", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := dc.Wiki(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] New wiki page \"index\" (Wiki change comment)", pl.Embeds[0].Title)
		assert.Equal(t, "Wiki change comment", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)

		p.Action = api.HookWikiEdited
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Wiki page \"index\" edited (Wiki change comment)", pl.Embeds[0].Title)
		assert.Equal(t, "Wiki change comment", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)

		p.Action = api.HookWikiDeleted
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Wiki page \"index\" deleted", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := dc.Release(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Release created: v1.0", pl.Embeds[0].Title)
		assert.Equal(t, "Note of first stable release", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/releases/tag/v1.0", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("WorkflowRun", func(t *testing.T) {
		testCases := []struct {
			runStatus      actions_model.Status
			expectedColour int
			expectedTitle  string
			expectedText   string
		}{
			{
				runStatus:      actions_model.StatusBlocked,
				expectedColour: yellowColor,
				expectedTitle:  `[acme/test] Workflow run "Update README.md" is blocked`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus:      actions_model.StatusCancelled,
				expectedColour: greyColor,
				expectedTitle:  `[acme/test] Workflow run "Update README.md" was cancelled`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus:      actions_model.StatusFailure,
				expectedColour: redColor,
				expectedTitle:  `[acme/test] Workflow run "Update README.md" has failed`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus:      actions_model.StatusRunning,
				expectedColour: greenColorLight,
				expectedTitle:  `[acme/test] Workflow run "Update README.md" has started running`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus:      actions_model.StatusSkipped,
				expectedColour: greyColor,
				expectedTitle:  `[acme/test] Workflow run "Update README.md" was skipped`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus:      actions_model.StatusSuccess,
				expectedColour: greenColor,
				expectedTitle:  `[acme/test] Workflow run "Update README.md" has completed successfully`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus:      actions_model.StatusWaiting,
				expectedColour: blueColor,
				expectedTitle:  `[acme/test] Workflow run "Update README.md" is waiting`,
				expectedText: `Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
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

				payload, err := dc.WorkflowRun(inputPayload)
				require.NoError(t, err)

				assert.Len(t, payload.Embeds, 1)
				assert.Equal(t, testCase.expectedColour, payload.Embeds[0].Color)
				assert.Equal(t, testCase.expectedTitle, payload.Embeds[0].Title)
				assert.Equal(t, testCase.expectedText, payload.Embeds[0].Description)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/197719",
					payload.Embeds[0].URL)
				assert.Equal(t, "jane", payload.Embeds[0].Author.Name)
				assert.Equal(t, setting.AppURL+"jane", payload.Embeds[0].Author.URL)
				assert.Equal(t, "https://example.com/avatars/7dc9cf?size=64", payload.Embeds[0].Author.IconURL)
			})
		}
	})

	t.Run("WorkflowJob", func(t *testing.T) {
		testCases := []struct {
			jobStatus      actions_model.Status
			expectedColour int
			expectedTitle  string
			expectedText   string
		}{
			{
				jobStatus:      actions_model.StatusBlocked,
				expectedColour: yellowColor,
				expectedTitle:  `[acme/test] Workflow job "build-and-test" is blocked`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus:      actions_model.StatusCancelled,
				expectedColour: greyColor,
				expectedTitle:  `[acme/test] Workflow job "build-and-test" was cancelled`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus:      actions_model.StatusFailure,
				expectedColour: redColor,
				expectedTitle:  `[acme/test] Workflow job "build-and-test" has failed`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus:      actions_model.StatusRunning,
				expectedColour: greenColorLight,
				expectedTitle:  `[acme/test] Workflow job "build-and-test" has started running`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus:      actions_model.StatusSkipped,
				expectedColour: greyColor,
				expectedTitle:  `[acme/test] Workflow job "build-and-test" was skipped`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus:      actions_model.StatusSuccess,
				expectedColour: greenColor,
				expectedTitle:  `[acme/test] Workflow job "build-and-test" has completed successfully`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus:      actions_model.StatusWaiting,
				expectedColour: blueColor,
				expectedTitle:  `[acme/test] Workflow job "build-and-test" is waiting`,
				expectedText: `Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
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
					},
				}

				payload, err := dc.WorkflowJob(inputPayload)
				require.NoError(t, err)

				assert.Len(t, payload.Embeds, 1)
				assert.Equal(t, testCase.expectedColour, payload.Embeds[0].Color)
				assert.Equal(t, testCase.expectedTitle, payload.Embeds[0].Title)
				assert.Equal(t, testCase.expectedText, payload.Embeds[0].Description)
				assert.Equal(t, "https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1",
					payload.Embeds[0].URL)
				assert.Equal(t, "jane", payload.Embeds[0].Author.Name)
				assert.Equal(t, setting.AppURL+"jane", payload.Embeds[0].Author.URL)
				assert.Equal(t, "https://example.com/avatars/7dc9cf?size=64", payload.Embeds[0].Author.IconURL)
			})
		}
	})
}

func TestDiscordJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.DISCORD,
		URL:        "https://discord.example.com/",
		Meta:       `{}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := discordHandler{}.NewRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://discord.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))

	buf, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	// https://codeberg.org/forgejo/forgejo/issues/11573
	assert.NotContains(t, string(buf), `"footer":`)

	var body DiscordPayload
	err = json.NewDecoder(bytes.NewReader(buf)).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message \\- user1\n[`2020558`](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message \\- user1", body.Embeds[0].Description)
}

var escapedMarkdownTests = map[string]struct {
	input    string
	expected string
}{
	"Escape heading level 1": {
		input:    "# Heading level 1",
		expected: "\\# Heading level 1",
	},
	"Escape heading level 2": {
		input:    "## Heading level 2",
		expected: "\\#\\# Heading level 2",
	},
	"Escape heading level 3": {
		input:    "### Heading level 3",
		expected: "\\#\\#\\# Heading level 3",
	},
	"Escape bold text": {
		input:    "**bold text**",
		expected: "\\*\\*bold text\\*\\*",
	},
	"Escape italic text": {
		input:    "*italic text*",
		expected: "\\*italic text\\*",
	},
	"Escape italic text underline": {
		input:    "_italic text_",
		expected: "\\_italic text\\_",
	},
	"Escape strikethrough": {
		input:    "~~strikethrough~~",
		expected: "\\~\\~strikethrough\\~\\~",
	},
	"Escape Ordered list item": {
		input:    "1. Ordered list item\n2. Second ordered list item\n999999999999. 999999999999 ordered list item",
		expected: "1\\. Ordered list item\n2\\. Second ordered list item\n999999999999\\. 999999999999 ordered list item",
	},
	"Escape Unordered list item": {
		input:    "- Unordered list\n + using plus",
		expected: "\\- Unordered list\n \\+ using plus",
	},
	"Escape bullet list item": {
		input:    "* Bullet list item",
		expected: "\\* Bullet list item",
	},
	"Escape table": {
		input:    "| Table | Example |\n|-|-|\n| Lorem | Ipsum |",
		expected: "\\| Table \\| Example \\|\n\\|-\\|-\\|\n\\| Lorem \\| Ipsum \\|",
	},
	"Escape link": {
		input:    "[Link to Forgejo](https://forgejo.org/)",
		expected: "\\[Link to Forgejo\\]\\(https://forgejo.org/\\)",
	},
	"Escape Alt text for an image": {
		input:    "![Alt text for an image](https://forgejo.org/_astro/mascot-dark.1omhhgvT_Zm0N2n.webp)",
		expected: "\\!\\[Alt text for an image\\]\\(https://forgejo.org/\\_astro/mascot-dark.1omhhgvT\\_Zm0N2n.webp\\)",
	},
	"Escape URL if it has markdown character": {
		input:    "https://forgejo.org/_astro/mascot-dark.1omhhgvT_Zm0N2n.webp",
		expected: "https://forgejo.org/\\_astro/mascot-dark.1omhhgvT\\_Zm0N2n.webp",
	},
	"Escape blockquote text": {
		input:    "> Blockquote text.",
		expected: "\\> Blockquote text.",
	},
	"Escape inline code": {
		input:    "`Inline code`",
		expected: "\\`Inline code\\`",
	},
	"Escape multiple code": {
		input:    "```\nCode block\nwith multiple lines\n```\n",
		expected: "\\`\\`\\`\nCode block\nwith multiple lines\n\\`\\`\\`\n",
	},
	"Escape horizontal rule": {
		input:    "---",
		expected: "\\---",
	},
}

func TestEscapeMarkdownChar(t *testing.T) {
	for name, test := range escapedMarkdownTests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, escapeMarkdown(test.input))
		})
	}
}
