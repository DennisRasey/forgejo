// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"errors"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/repo"
	"forgejo.org/models/user"
	"forgejo.org/modules/webhook"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
	act_model "code.forgejo.org/forgejo/runner/v13/act/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigureActionRunTitle(t *testing.T) {
	const defaultTitle = "default title"
	for _, tc := range []struct {
		name          string
		workflow      string
		expectedTitle string
	}{
		{
			name: "no run-name keeps default title",
			workflow: `
on: push
jobs:
  job:
    runs-on: ubuntu
    steps: []
`,
			expectedTitle: defaultTitle,
		},
		{
			name: "run-name is used",
			workflow: `
run-name: "good title"
on: push
jobs:
  job:
    runs-on: ubuntu
    steps: []
`,
			expectedTitle: "good title",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := &actions_model.ActionRun{
				Title:        defaultTitle,
				Ref:          "refs/head/main",
				WorkflowID:   "testing.yml",
				Event:        webhook.HookEventPush,
				TriggerEvent: string(webhook.HookEventPush),
				TriggerUser:  &user.User{Name: "someone"},
				Repo:         &repo.Repository{},
			}
			workflows, err := jobparser.Parse([]byte(tc.workflow), false)
			require.NoError(t, err)

			require.NoError(t, ConfigureActionRunTitle(workflows, run))

			assert.Equal(t, tc.expectedTitle, run.Title)
		})
	}
}

func TestConfigureActionRunConcurrency(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		concurrency              *act_model.RawConcurrency
		vars                     map[string]string
		inputs                   map[string]any
		runEvent                 webhook.HookEventType
		expectedConcurrencyGroup string
		expectedConcurrencyType  actions_model.ConcurrencyMode
	}{
		// Before the introduction of concurrency groups, push & pull_request_sync would cancel runs on the same repo,
		// reference, workflow, and event -- these cases cover undefined concurrency group and backwards compatibility
		// checks.
		{
			name:                     "backwards compatibility push",
			runEvent:                 webhook.HookEventPush,
			expectedConcurrencyGroup: "refs/head/main_testing.yml_push__auto",
			expectedConcurrencyType:  actions_model.CancelInProgress,
		},
		{
			name:                     "backwards compatibility pull_request_sync",
			runEvent:                 webhook.HookEventPullRequestSync,
			expectedConcurrencyGroup: "refs/head/main_testing.yml_pull_request_sync__auto",
			expectedConcurrencyType:  actions_model.CancelInProgress,
		},
		{
			name:                     "backwards compatibility other event",
			runEvent:                 webhook.HookEventWorkflowDispatch,
			expectedConcurrencyGroup: "refs/head/main_testing.yml_workflow_dispatch__auto",
			expectedConcurrencyType:  actions_model.UnlimitedConcurrency,
		},

		{
			name: "fully-specified cancel-in-progress",
			concurrency: &act_model.RawConcurrency{
				Group:            "abc",
				CancelInProgress: "true",
			},
			runEvent:                 webhook.HookEventPullRequestSync,
			expectedConcurrencyGroup: "abc",
			expectedConcurrencyType:  actions_model.CancelInProgress,
		},
		{
			name: "fully-specified queue-behind",
			concurrency: &act_model.RawConcurrency{
				Group:            "abc",
				CancelInProgress: "false",
			},
			runEvent:                 webhook.HookEventPullRequestSync,
			expectedConcurrencyGroup: "abc",
			expectedConcurrencyType:  actions_model.QueueBehind,
		},
		{
			name: "no concurrency group, cancel-in-progress: false",
			concurrency: &act_model.RawConcurrency{
				CancelInProgress: "false",
			},
			runEvent:                 webhook.HookEventPullRequestSync,
			expectedConcurrencyGroup: "refs/head/main_testing.yml_pull_request_sync__auto",
			expectedConcurrencyType:  actions_model.UnlimitedConcurrency,
		},

		{
			name: "interpreted values",
			concurrency: &act_model.RawConcurrency{
				Group:            "${{ github.workflow }}-${{ github.ref }}",
				CancelInProgress: "${{ !contains(github.ref, 'release/')}}",
			},
			runEvent:                 webhook.HookEventPullRequestSync,
			expectedConcurrencyGroup: "testing.yml-refs/head/main",
			expectedConcurrencyType:  actions_model.CancelInProgress,
		},
		{
			name: "interpreted values with inputs and vars",
			concurrency: &act_model.RawConcurrency{
				Group: "${{ inputs.abc }}-${{ vars.def }}",
			},
			inputs:                   map[string]any{"abc": "123"},
			vars:                     map[string]string{"def": "456"},
			runEvent:                 webhook.HookEventPullRequestSync,
			expectedConcurrencyGroup: "123-456",
			expectedConcurrencyType:  actions_model.CancelInProgress,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workflow := &act_model.Workflow{RawConcurrency: tc.concurrency}
			run := &actions_model.ActionRun{
				Ref:          "refs/head/main",
				WorkflowID:   "testing.yml",
				Event:        tc.runEvent,
				TriggerEvent: string(tc.runEvent),
				Repo:         &repo.Repository{},
			}

			err := ConfigureActionRunConcurrency(workflow, run, tc.vars, tc.inputs)
			require.NoError(t, err)

			if tc.expectedConcurrencyGroup == "" {
				assert.Empty(t, run.ConcurrencyGroup, "empty ConcurrencyGroup")
			} else {
				assert.Equal(t, tc.expectedConcurrencyGroup, run.ConcurrencyGroup)
			}
			assert.Equal(t, tc.expectedConcurrencyType, run.ConcurrencyType)
		})
	}
}

func TestResolveDispatchInputAcceptsValidInput(t *testing.T) {
	for _, tc := range []struct {
		name          string
		key           string
		value         string
		input         act_model.WorkflowDispatchInput
		expected      string
		expectedError func(err error) bool
	}{
		{
			name:     "on_converted_to_true",
			key:      "my_boolean",
			value:    "on",
			input:    act_model.WorkflowDispatchInput{Description: "a boolean", Required: false, Type: "boolean", Options: []string{}},
			expected: "true",
		},
		// It might make sense to validate booleans in the future and then turn it into an error.
		{
			name:     "ON_stays_ON",
			key:      "my_boolean",
			value:    "ON",
			input:    act_model.WorkflowDispatchInput{Description: "a boolean", Required: false, Type: "boolean", Options: []string{}},
			expected: "ON",
		},
		{
			name:     "true_stays_true",
			key:      "my_boolean",
			value:    "true",
			input:    act_model.WorkflowDispatchInput{Description: "a boolean", Required: false, Type: "boolean", Options: []string{}},
			expected: "true",
		},
		{
			name:     "false_stays_false",
			key:      "my_boolean",
			value:    "false",
			input:    act_model.WorkflowDispatchInput{Description: "a boolean", Required: false, Type: "boolean", Options: []string{}},
			expected: "false",
		},
		{
			name:     "empty_results_in_default_value_true",
			key:      "my_boolean",
			value:    "",
			input:    act_model.WorkflowDispatchInput{Description: "a boolean", Required: false, Default: "true", Type: "boolean", Options: []string{}},
			expected: "true",
		},
		{
			name:     "empty_results_in_default_value_false",
			key:      "my_boolean",
			value:    "",
			input:    act_model.WorkflowDispatchInput{Description: "a boolean", Required: false, Default: "false", Type: "boolean", Options: []string{}},
			expected: "false",
		},
		{
			name:     "string_results_in_input",
			key:      "my_string",
			value:    "hello",
			input:    act_model.WorkflowDispatchInput{Description: "a string", Required: false, Type: "string", Options: []string{}},
			expected: "hello",
		},
		{
			name:     "string_option_results_in_input",
			value:    "a",
			input:    act_model.WorkflowDispatchInput{Description: "a string", Required: false, Type: "string", Options: []string{"a", "b"}},
			expected: "a",
		},
		// Test ensures that the old behaviour (ignoring option mismatch) is retained. It might
		// make sense to turn it into an error in the future.
		{
			name:     "invalid_string_option_results_in_input",
			key:      "option",
			value:    "c",
			input:    act_model.WorkflowDispatchInput{Description: "a string", Required: false, Type: "string", Options: []string{"a", "b"}},
			expected: "c",
		},
		{
			name:     "number_results_in_input",
			key:      "my_number",
			value:    "123",
			input:    act_model.WorkflowDispatchInput{Description: "a string", Required: false, Type: "number", Options: []string{}},
			expected: "123",
		},
		{
			name:          "empty_value_skipped",
			key:           "my_number",
			value:         "",
			input:         act_model.WorkflowDispatchInput{Description: "a string", Required: false, Type: "number", Options: []string{}},
			expectedError: func(err error) bool { return errors.Is(err, ErrSkipDispatchInput) },
		},
		{
			name:          "required_missing",
			key:           "my_number",
			value:         "",
			input:         act_model.WorkflowDispatchInput{Required: true, Type: "number", Options: []string{}},
			expectedError: IsInputRequiredErr,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := resolveDispatchInput(tc.key, tc.value, tc.input)
			if tc.expectedError != nil {
				assert.True(t, tc.expectedError(err))
			} else {
				assert.Equal(t, tc.expected, actual)
			}
		})
	}
}

func TestResolveDispatchInputRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name     string
		key      string
		value    string
		input    act_model.WorkflowDispatchInput
		expected error
	}{
		{
			name:     "missing_required_boolean",
			key:      "missing_boolean",
			value:    "",
			input:    act_model.WorkflowDispatchInput{Description: "a boolean", Required: true, Type: "boolean", Options: []string{}},
			expected: InputRequiredErr{Name: "a boolean"},
		},
		{
			name:     "missing_required_boolean_without_description",
			key:      "missing_boolean",
			value:    "",
			input:    act_model.WorkflowDispatchInput{Required: true, Type: "boolean", Options: []string{}},
			expected: InputRequiredErr{Name: "missing_boolean"},
		},
		{
			name:     "missing_required_string",
			key:      "missing_string",
			value:    "",
			input:    act_model.WorkflowDispatchInput{Description: "a string", Required: true, Type: "string", Options: []string{}},
			expected: InputRequiredErr{Name: "a string"},
		},
		{
			name:     "missing_required_string_without_description",
			key:      "missing_string",
			value:    "",
			input:    act_model.WorkflowDispatchInput{Required: true, Type: "string", Options: []string{}},
			expected: InputRequiredErr{Name: "missing_string"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveDispatchInput(tc.key, tc.value, tc.input)
			assert.Equal(t, tc.expected, err)
		})
	}
}
