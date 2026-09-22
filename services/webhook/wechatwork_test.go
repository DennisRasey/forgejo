// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package webhook

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	api "forgejo.org/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWechatWorkPayload(t *testing.T) {
	wc := wechatworkConvertor{}

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

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus: actions_model.StatusCancelled,
				expectedText: `[acme/test] Workflow run "Update README.md" was cancelled

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus: actions_model.StatusFailure,
				expectedText: `[acme/test] Workflow run "Update README.md" has failed

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus: actions_model.StatusRunning,
				expectedText: `[acme/test] Workflow run "Update README.md" has started running

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus: actions_model.StatusSkipped,
				expectedText: `[acme/test] Workflow run "Update README.md" was skipped

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus: actions_model.StatusSuccess,
				expectedText: `[acme/test] Workflow run "Update README.md" has completed successfully

Repository: acme/test
Run: Update README.md

View details on https://example.com/acme/test/actions/runs/197719.
`,
			},
			{
				runStatus: actions_model.StatusWaiting,
				expectedText: `[acme/test] Workflow run "Update README.md" is waiting

Repository: acme/test
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

				payload, err := wc.WorkflowRun(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedText, payload.Markdown.Content)
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

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus: actions_model.StatusCancelled,
				expectedText: `[acme/test] Workflow job "build-and-test" was cancelled

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus: actions_model.StatusFailure,
				expectedText: `[acme/test] Workflow job "build-and-test" has failed

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus: actions_model.StatusRunning,
				expectedText: `[acme/test] Workflow job "build-and-test" has started running

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus: actions_model.StatusSkipped,
				expectedText: `[acme/test] Workflow job "build-and-test" was skipped

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus: actions_model.StatusSuccess,
				expectedText: `[acme/test] Workflow job "build-and-test" has completed successfully

Repository: acme/test
Run: Update README.md
Job: build-and-test

View details on https://example.com/acme/test/actions/runs/196540/jobs/3/attempt/1.
`,
			},
			{
				jobStatus: actions_model.StatusWaiting,
				expectedText: `[acme/test] Workflow job "build-and-test" is waiting

Repository: acme/test
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
						Repo: &api.Repository{
							FullName: "acme/test",
						},
						TriggerUser: &api.User{
							UserName:  "jane",
							AvatarURL: "https://example.com/avatars/7dc9cf?size=64",
						},
					},
				}

				payload, err := wc.WorkflowJob(inputPayload)
				require.NoError(t, err)

				assert.Equal(t, testCase.expectedText, payload.Markdown.Content)
			})
		}
	})
}
