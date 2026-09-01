// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"slices"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/test"
	notify_service "forgejo.org/services/notify"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
	"code.forgejo.org/forgejo/runner/v13/act/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func Test_jobStatusResolver_Resolve(t *testing.T) {
	tests := []struct {
		name string
		jobs actions_model.ActionJobList
		want map[int64]actions_model.Status
	}{
		{
			name: "no blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusWaiting, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusWaiting, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 3, JobID: "3", Status: actions_model.StatusWaiting, Needs: []actions_model.LocalJobIdentifier{}},
			},
			want: map[int64]actions_model.Status{},
		},
		{
			name: "single blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusWaiting, Needs: []actions_model.LocalJobIdentifier{}},
			},
			want: map[int64]actions_model.Status{
				2: actions_model.StatusWaiting,
			},
		},
		{
			name: "multiple blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"1"}},
			},
			want: map[int64]actions_model.Status{
				2: actions_model.StatusWaiting,
				3: actions_model.StatusWaiting,
			},
		},
		{
			name: "chain blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusFailure, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"2"}},
			},
			want: map[int64]actions_model.Status{
				// Resolve() does only one update pass and does not update jobs recursively. Therefore, job 3, which
				// depends on 2, is not marked as waiting. It would only be marked as waiting if it depended on job 1.
				2: actions_model.StatusWaiting,
			},
		},
		{
			name: "loop need",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"3"}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"2"}},
			},
			want: map[int64]actions_model.Status{},
		},
		{
			name: "`if` is not empty and all jobs in `needs` completed successfully",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job1", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "job2", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"job1"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    runs-on: ubuntu-latest
    needs: job1
    if: ${{ always() && needs.job1.result == 'success' }}
    steps:
      - run: echo "will be checked by act_runner"
`,
				)},
			},
			want: map[int64]actions_model.Status{2: actions_model.StatusWaiting},
		},
		{
			name: "`if` is not empty and not all jobs in `needs` completed successfully",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job1", Status: actions_model.StatusFailure, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "job2", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"job1"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    runs-on: ubuntu-latest
    needs: job1
    if: ${{ always() && needs.job1.result == 'failure' }}
    steps:
      - run: echo "will be checked by act_runner"
`,
				)},
			},
			want: map[int64]actions_model.Status{2: actions_model.StatusWaiting},
		},
		{
			name: "`if` is empty and not all jobs in `needs` completed successfully",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job1", Status: actions_model.StatusFailure, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "job2", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"job1"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    runs-on: ubuntu-latest
    needs: job1
    steps:
      - run: echo "should be skipped"
`,
				)},
			},
			// Status goes to waiting for `prepareJobForEmitting` to evaluate `if`, even in default condition, so one
			// codepath always handles this consistently.
			want: map[int64]actions_model.Status{2: actions_model.StatusWaiting},
		},
		{
			name: "unblocked workflow call outer job with success",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobNamespace: "ns1", JobID: "innerjob1", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobNamespace: "ns1", JobID: "innerjob2", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 3, JobID: "job1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"__namespace.ns1.innerjob1", "__namespace.ns1.innerjob2"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    if: false
    uses: ./.forgejo/workflows/reusable.yml
__metadata:
  workflow_call_id: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed
`,
				)},
			},
			want: map[int64]actions_model.Status{
				3: actions_model.StatusSuccess,
			},
		},
		{
			name: "unblocked workflow call outer job with success and skip",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobNamespace: "ns1", JobID: "innerjob1", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobNamespace: "ns1", JobID: "innerjob2", Status: actions_model.StatusSkipped, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 3, JobID: "job1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"__namespace.ns1.innerjob1", "__namespace.ns1.innerjob2"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    if: false
    uses: ./.forgejo/workflows/reusable.yml
__metadata:
  workflow_call_id: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed
`,
				)},
			},
			want: map[int64]actions_model.Status{
				3: actions_model.StatusSuccess,
			},
		},
		{
			name: "unblocked workflow call outer job with only skip",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobNamespace: "ns1", JobID: "innerjob1", Status: actions_model.StatusSkipped, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobNamespace: "ns1", JobID: "innerjob2", Status: actions_model.StatusSkipped, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 3, JobID: "job1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"__namespace.ns1.innerjob1", "__namespace.ns1.innerjob2"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    if: false
    uses: ./.forgejo/workflows/reusable.yml
__metadata:
  workflow_call_id: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed
`,
				)},
			},
			want: map[int64]actions_model.Status{
				3: actions_model.StatusSkipped,
			},
		},
		{
			name: "unblocked workflow call outer job, incomplete `with`",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job0", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "job1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"job0"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    if: false
    uses: ./.forgejo/workflows/reusable.yml
    with:
      something: ${{ needs.job0.outputs.something }}
incomplete_with: true
incomplete_with_needs:
  job: job0
  output: something
__metadata:
  workflow_call_id: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed
`,
				)},
			},
			want: map[int64]actions_model.Status{
				2: actions_model.StatusWaiting,
			},
		},
		{
			name: "unblocked workflow call outer job, incomplete `strategy.matrix`",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job0", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "job1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"job0"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    if: false
    uses: ./.forgejo/workflows/reusable.yml
    strategy:
      matrix: ${{ fromJSON(needs.job0.outputs.something) }}
incomplete_matrix: true
incomplete_matrix_needs:
  job: job0
  output: something
__metadata:
  workflow_call_id: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed
`,
				)},
			},
			want: map[int64]actions_model.Status{
				2: actions_model.StatusWaiting,
			},
		},
		{
			name: "unblocked workflow call outer job with internal failure",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobNamespace: "ns1", JobID: "innerjob1", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobNamespace: "ns1", JobID: "innerjob2", Status: actions_model.StatusFailure, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 3, JobID: "job1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"__namespace.ns1.innerjob1", "__namespace.ns1.innerjob2"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    if: false
    uses: ./.forgejo/workflows/reusable.yml
__metadata:
  workflow_call_id: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed
`,
				)},
			},
			want: map[int64]actions_model.Status{
				3: actions_model.StatusFailure,
			},
		},
		{
			name: "unblocked workflow call outer job with internal skip & failure",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobNamespace: "ns1", JobID: "innerjob1", Status: actions_model.StatusSkipped, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobNamespace: "ns1", JobID: "innerjob2", Status: actions_model.StatusFailure, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 3, JobID: "job1", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"__namespace.ns1.innerjob1", "__namespace.ns1.innerjob2"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    if: false
    uses: ./.forgejo/workflows/reusable.yml
__metadata:
  workflow_call_id: b5a9f46f1f2513d7777fde50b169d323a6519e349cc175484c947ac315a209ed
`,
				)},
			},
			want: map[int64]actions_model.Status{
				3: actions_model.StatusFailure,
			},
		},
		{
			name: "blocked if needs are unknown",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "build", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 2, JobID: "test", Status: actions_model.StatusBlocked, Needs: []actions_model.LocalJobIdentifier{"build", "unknown"}},
			},
			want: map[int64]actions_model.Status{},
		},
		{
			name: "blocked if needs are unknown despite always()",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "build", Status: actions_model.StatusSuccess, Needs: []actions_model.LocalJobIdentifier{}},
				{ID: 45, JobID: "test", Needs: []actions_model.LocalJobIdentifier{"build", "unknown"}, Status: actions_model.StatusBlocked, WorkflowPayload: []byte(`
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    needs: [build, unknown]
    if: always()
    steps: []
`)},
			},
			want: map[int64]actions_model.Status{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newJobStatusResolver(tt.jobs)
			assert.Equal(t, tt.want, r.Resolve())
		})
	}
}

const testWorkflowCallSimpleExpansion = `
on:
  workflow_call:
    inputs:
      workflow_input:
        type: string
jobs:
  inner_job:
    name: "inner ${{ inputs.workflow_input }}"
    runs-on: debian-latest
    steps:
      - run: echo ${{ inputs.workflow_input }}
`

const testWorkflowCallMoreIncompleteExpansion = `
on:
  workflow_call:
    inputs:
      workflow_input:
        type: string
jobs:
  define-runs-on:
    name: "inner define-runs-on ${{ inputs.workflow_input }}"
    runs-on: docker
    outputs:
      scalar-value: ${{ steps.define.outputs.scalar }}
    steps:
      - id: define
        run: |
          echo 'scalar=scalar value' >> "$FORGEJO_OUTPUT"
  scalar-job:
    name: "inner incomplete-job ${{ inputs.workflow_input }}"
    runs-on: ${{ needs.define-runs-on.outputs.scalar-value }}
    needs: define-runs-on
    steps: []
`

type mockNotifier struct {
	notify_service.NullNotifier
	events []actions_model.ActionRunEvent
}

func (m *mockNotifier) WorkflowRunEvent(_ context.Context, event actions_model.ActionRunEvent) {
	m.events = append(m.events, event)
}

func Test_prepareJobForEmitting(t *testing.T) {
	// Shouldn't get any decoding errors during this test -- pop them up from a log warning to a test fatal error.
	defer test.MockVariableValue(&model.OnDecodeNodeError, func(node yaml.Node, out any, err error) {
		t.Fatalf("Failed to decode node %v into %T: %v", node, out, err)
	})()

	type localReusableWorkflowCallArgs struct {
		repoID    int64
		commitSHA string
		path      string
	}

	tests := []struct {
		name                          string
		runJobID                      int64
		errContains                   string
		consumed                      bool
		runJobNames                   []string
		preExecutionError             actions_model.PreExecutionError
		preExecutionErrorDetails      []any
		runsOn                        map[string][]string
		needs                         map[string][]actions_model.LocalJobIdentifier
		expectIncompleteJob           []string
		localReusableWorkflowCallArgs *localReusableWorkflowCallArgs
		actionRunStatusChange         actions_model.Status
		notificationsVerifier         func(notifier *notify_service.MockNotifier)
	}{
		{
			name:        "matrix expanded to 3 new jobs",
			runJobID:    601,
			consumed:    true,
			runJobNames: []string{"define-matrix", "produce-artifacts (blue)", "produce-artifacts (green)", "produce-artifacts (red)"},
			needs: map[string][]actions_model.LocalJobIdentifier{
				"define-matrix":             nil,
				"produce-artifacts (blue)":  {"define-matrix"},
				"produce-artifacts (green)": {"define-matrix"},
				"produce-artifacts (red)":   {"define-matrix"},
			},
		},
		{
			name:        "needs an incomplete job",
			runJobID:    603,
			errContains: "jobStatusResolver attempted to prepareJobForEmitting for a job (id=603) with an incomplete 'needs' job (id=604)",
		},
		{
			name:                     "missing needs for strategy.matrix evaluation",
			runJobID:                 605,
			preExecutionError:        actions_model.ErrorCodeIncompleteMatrixMissingJob,
			preExecutionErrorDetails: []any{"produce-artifacts", "define-matrix-2", "define-matrix-1"},
		},
		{
			name:        "matrix expanded to 0 jobs",
			runJobID:    607,
			consumed:    true,
			runJobNames: []string{"define-matrix"},
		},
		{
			name:     "matrix multiple dimensions from separate outputs",
			runJobID: 609,
			consumed: true,
			runJobNames: []string{
				"define-matrix",
				"run-tests (site-a, 12.x, 17)",
				"run-tests (site-a, 12.x, 18)",
				"run-tests (site-a, 14.x, 17)",
				"run-tests (site-a, 14.x, 18)",
				"run-tests (site-b, 12.x, 17)",
				"run-tests (site-b, 12.x, 18)",
				"run-tests (site-b, 14.x, 17)",
				"run-tests (site-b, 14.x, 18)",
			},
		},
		{
			name:     "matrix multiple dimensions from one output",
			runJobID: 611,
			consumed: true,
			runJobNames: []string{
				"define-matrix",
				"run-tests (site-a, 12.x, 17)",
				"run-tests (site-a, 12.x, 18)",
				"run-tests (site-a, 14.x, 17)",
				"run-tests (site-a, 14.x, 18)",
				"run-tests (site-b, 12.x, 17)",
				"run-tests (site-b, 12.x, 18)",
				"run-tests (site-b, 14.x, 17)",
				"run-tests (site-b, 14.x, 18)",
			},
		},
		{
			// This test case also includes `on: [push]` in the workflow_payload, which appears to trigger a regression
			// in go.yaml.in/yaml/v4 v4.0.0-rc.2 (which I had accidentally referenced in job_emitter.go), and so serves
			// as a regression prevention test for this case...
			//
			// unmarshal WorkflowPayload to SingleWorkflow failed: yaml: unmarshal errors: line 1: cannot unmarshal
			// !!seq into yaml.Node
			name:     "scalar expansion into matrix",
			runJobID: 613,
			consumed: true,
			runJobNames: []string{
				"define-matrix",
				"scalar-job (hard-coded value)",
				"scalar-job (just some value)",
			},
		},
		{
			name:                     "missing needs output for strategy.matrix evaluation",
			runJobID:                 615,
			preExecutionError:        actions_model.ErrorCodeIncompleteMatrixMissingOutput,
			preExecutionErrorDetails: []any{"produce-artifacts", "define-matrix-1", "colours-intentional-mistake"},
		},
		{
			name:     "runs-on evaluation with needs",
			runJobID: 617,
			consumed: true,
			runJobNames: []string{
				"consume-runs-on",
				"define-runs-on",
			},
			runsOn: map[string][]string{
				"define-runs-on":  {"fedora"},
				"consume-runs-on": {"nixos-25.11"},
			},
		},
		{
			name:     "runs-on evaluation with needs dynamic matrix",
			runJobID: 619,
			consumed: true,
			runJobNames: []string{
				"consume-runs-on (site-a, 12.x, 17)",
				"consume-runs-on (site-a, 12.x, 18)",
				"consume-runs-on (site-a, 14.x, 17)",
				"consume-runs-on (site-a, 14.x, 18)",
				"consume-runs-on (site-b, 12.x, 17)",
				"consume-runs-on (site-b, 12.x, 18)",
				"consume-runs-on (site-b, 14.x, 17)",
				"consume-runs-on (site-b, 14.x, 18)",
				"define-matrix",
			},
			runsOn: map[string][]string{
				"consume-runs-on (site-a, 12.x, 17)": {"node-12.x"},
				"consume-runs-on (site-a, 12.x, 18)": {"node-12.x"},
				"consume-runs-on (site-a, 14.x, 17)": {"node-14.x"},
				"consume-runs-on (site-a, 14.x, 18)": {"node-14.x"},
				"consume-runs-on (site-b, 12.x, 17)": {"node-12.x"},
				"consume-runs-on (site-b, 12.x, 18)": {"node-12.x"},
				"consume-runs-on (site-b, 14.x, 17)": {"node-14.x"},
				"consume-runs-on (site-b, 14.x, 18)": {"node-14.x"},
				"define-matrix":                      {"fedora"},
			},
		},
		{
			name:     "runs-on evaluation to part of array",
			runJobID: 621,
			consumed: true,
			runJobNames: []string{
				"consume-runs-on",
				"define-runs-on",
			},
			runsOn: map[string][]string{
				"define-runs-on": {"fedora"},
				"consume-runs-on": {
					"datacenter-alpha",
					"nixos-25.11",
					"node-27.x",
				},
			},
		},
		{
			name:                     "missing needs job for runs-on evaluation",
			runJobID:                 623,
			preExecutionError:        actions_model.ErrorCodeIncompleteRunsOnMissingJob,
			preExecutionErrorDetails: []any{"consume-runs-on", "oops-i-misspelt-the-job-id", "define-runs-on, another-needs"},
		},
		{
			name:                     "missing needs output for runs-on evaluation",
			runJobID:                 625,
			preExecutionError:        actions_model.ErrorCodeIncompleteRunsOnMissingOutput,
			preExecutionErrorDetails: []any{"consume-runs-on", "define-runs-on", "output-doesnt-exist"},
		},
		{
			name:                     "missing matrix dimension for runs-on evaluation",
			runJobID:                 627,
			preExecutionError:        actions_model.ErrorCodeIncompleteRunsOnMissingMatrixDimension,
			preExecutionErrorDetails: []any{"consume-runs-on", "dimension-oops-error"},
		},
		{
			name:                     "workflow call remote reference unavailable",
			runJobID:                 629,
			preExecutionError:        actions_model.ErrorCodeJobParsingError,
			preExecutionErrorDetails: []any{"unable to read instance workflow \"some-repo/some-org/.forgejo/workflows/reusable.yml@non-existent-reference\": someone deleted that reference maybe"},
		},
		{
			name:     "workflow call with needs expansion",
			runJobID: 630,
			consumed: true,
			runJobNames: []string{
				"define-workflow-call",
				"inner my-workflow-input",
				"perform-workflow-call",
			},
			needs: map[string][]actions_model.LocalJobIdentifier{
				"define-workflow-call":    nil,
				"inner my-workflow-input": {"__namespace..define-workflow-call"},
				"perform-workflow-call":   {"define-workflow-call", "__namespace.c899b6b202f3150372a11cf825e2c643434dc255afc960fadec5e9298eb8cac1.inner_job"},
			},
		},
		// Before reusable workflow expansion, there weren't any cases where evaluating a job in the job emitter could
		// result in more incomplete jobs being generated (other than errors).  This is the first such case -- run job
		// ID 632 references reusable workflow "more-incomplete" which generates more incomplete jobs.
		{
			name:     "workflow call generates more incomplete jobs",
			runJobID: 632,
			consumed: true,
			runJobNames: []string{
				"define-workflow-call",
				"inner define-runs-on my-workflow-input",
				"inner incomplete-job my-workflow-input",
				"perform-workflow-call",
			},
			runsOn: map[string][]string{
				"define-workflow-call":                   {"fedora"},
				"perform-workflow-call":                  {},
				"inner define-runs-on my-workflow-input": {"docker"},
				"inner incomplete-job my-workflow-input": {"${{ needs.define-runs-on.outputs.scalar-value }}"},
			},
			needs: map[string][]actions_model.LocalJobIdentifier{
				"define-workflow-call":                   nil,
				"inner define-runs-on my-workflow-input": {"__namespace..define-workflow-call"},
				"inner incomplete-job my-workflow-input": {"__namespace..define-workflow-call", "define-runs-on"},
				"perform-workflow-call": {
					"define-workflow-call",
					"__namespace.c899b6b202f3150372a11cf825e2c643434dc255afc960fadec5e9298eb8cac1.define-runs-on",
					"__namespace.c899b6b202f3150372a11cf825e2c643434dc255afc960fadec5e9298eb8cac1.scalar-job",
				},
			},
			expectIncompleteJob: []string{"inner incomplete-job my-workflow-input"},
		},
		{
			name:                     "missing needs job for workflow call evaluation",
			runJobID:                 634,
			preExecutionError:        actions_model.ErrorCodeIncompleteWithMissingJob,
			preExecutionErrorDetails: []any{"perform-workflow-call", "oops-i-misspelt-the-job-id", "define-workflow-call"},
			notificationsVerifier: func(notifier *notify_service.MockNotifier) {
				notifier.AssertNumberOfCalls(t, "NewWorkflowJobAttempt", 0)
				notifier.AssertNumberOfCalls(t, "WorkflowJobStatusChanged", 0)
				notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 0)
				notifier.AssertNumberOfCalls(t, "WorkflowRunEvent", 1)

				notifier.AssertCalled(
					t, "WorkflowRunEvent", mock.Anything,
					mock.MatchedBy(func(event *actions_model.WorkflowRunCompleted) bool {
						return event.GetRun().Status == actions_model.StatusFailure
					}),
				)
			},
		},
		{
			name:                     "missing needs output for workflow call evaluation",
			runJobID:                 636,
			preExecutionError:        actions_model.ErrorCodeIncompleteWithMissingOutput,
			preExecutionErrorDetails: []any{"perform-workflow-call", "define-workflow-call", "output-doesnt-exist"},
		},
		{
			name:                     "missing matrix dimension for workflow call evaluation",
			runJobID:                 638,
			preExecutionError:        actions_model.ErrorCodeIncompleteWithMissingMatrixDimension,
			preExecutionErrorDetails: []any{"perform-workflow-call", "dimension-oops-error"},
		},
		{
			name:     "local workflow call with needs expansion",
			runJobID: 640,
			consumed: true,
			runJobNames: []string{
				"define-workflow-call",
				"inner my-workflow-input",
				"perform-workflow-call",
			},
			localReusableWorkflowCallArgs: &localReusableWorkflowCallArgs{
				repoID:    63,
				commitSHA: "97f29ee599c373c729132a5c46a046978311e0ee",
				path:      "./.forgejo/workflows/reusable.yml",
			},
		},
		{
			name:     "action run completed after expansion",
			runJobID: 642,
			consumed: true,
			runJobNames: []string{
				"job1",
				// job2 which expanded into an empty matrix is gone
			},
			actionRunStatusChange: actions_model.StatusSuccess,
			notificationsVerifier: func(notifier *notify_service.MockNotifier) {
				notifier.AssertNumberOfCalls(t, "NewWorkflowJobAttempt", 0)
				notifier.AssertNumberOfCalls(t, "WorkflowJobStatusChanged", 0)
				notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 0)
				notifier.AssertNumberOfCalls(t, "WorkflowRunEvent", 1)
				notifier.AssertCalled(
					t, "WorkflowRunEvent", mock.Anything,
					mock.MatchedBy(func(event *actions_model.WorkflowRunCompleted) bool {
						return event.GetRun().Status == actions_model.StatusSuccess
					}),
				)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer unittest.OverrideFixtures("services/actions/Test_prepareJobForEmitting")()
			require.NoError(t, unittest.PrepareTestDatabase())

			notifier := notify_service.NewMockNotifier(t)
			notifier.On("Run").Return().Maybe()
			notifier.On("NewWorkflowJobAttempt", mock.Anything, mock.Anything).Return(nil).Maybe()
			notifier.On("WorkflowJobStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			notifier.On("WorkflowRunEvent", mock.Anything, mock.Anything).Return(nil).Maybe()

			notify_service.RegisterNotifier(notifier)
			defer notify_service.UnregisterNotifier(notifier)

			// Mock access to reusable workflows, both local and remote
			var localReusableCalled []*localReusableWorkflowCallArgs
			var cleanupCallCount int
			defer test.MockVariableValue(&lazyRepoExpandLocalReusableWorkflow,
				func(ctx context.Context, repoID int64, commitSHA string) (jobparser.LocalWorkflowFetcher, CleanupFunc) {
					fetcher := func(job *jobparser.Job, path string) ([]byte, error) {
						localReusableCalled = append(localReusableCalled, &localReusableWorkflowCallArgs{repoID, commitSHA, path})
						return []byte(testWorkflowCallSimpleExpansion), nil
					}
					cleanup := func() {
						cleanupCallCount++
					}
					return fetcher, cleanup
				})()
			defer test.MockVariableValue(&expandInstanceReusableWorkflows,
				func(ctx context.Context) jobparser.InstanceWorkflowFetcher {
					return func(job *jobparser.Job, ref *model.NonLocalReusableWorkflowReference) ([]byte, error) {
						switch ref.Ref {
						case "non-existent-reference":
							return nil, errors.New("someone deleted that reference maybe")
						case "simple":
							return []byte(testWorkflowCallSimpleExpansion), nil
						case "more-incomplete":
							return []byte(testWorkflowCallMoreIncompleteExpansion), nil
						}
						return nil, errors.New("unknown workflow reference")
					}
				})()

			blockedJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: tt.runJobID})

			jobsInRun, err := db.Find[actions_model.ActionRunJob](t.Context(), actions_model.FindRunJobOptions{RunID: blockedJob.RunID})
			require.NoError(t, err)

			behaviour, err := prepareJobForEmitting(t.Context(), blockedJob, jobsInRun)

			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
			} else {
				require.NoError(t, err)
				if tt.consumed {
					assert.Equal(t, behaviourIgnoreJob, behaviour)

					// blockedJob should no longer exist in the database
					unittest.AssertNotExistsBean(t, &actions_model.ActionRunJob{ID: tt.runJobID})

					// expectations are that the ActionRun has an empty PreExecutionError
					actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: blockedJob.RunID})
					assert.EqualValues(t, 0, actionRun.PreExecutionErrorCode, "PreExecutionError Details: %#v", actionRun.PreExecutionErrorDetails)
					if tt.actionRunStatusChange != 0 {
						assert.Equal(t, tt.actionRunStatusChange, actionRun.Status)
					}

					// compare jobs that exist with `runJobNames` to ensure new jobs are inserted:
					allJobsInRun, err := db.Find[actions_model.ActionRunJob](t.Context(), actions_model.FindRunJobOptions{RunID: blockedJob.RunID})
					require.NoError(t, err)
					allJobNames := []string{}
					for _, j := range allJobsInRun {
						allJobNames = append(allJobNames, j.Name)
					}
					slices.Sort(allJobNames)
					assert.Equal(t, tt.runJobNames, allJobNames)

					// Check the runs-on of all jobs
					if tt.runsOn != nil {
						for _, j := range allJobsInRun {
							expected, ok := tt.runsOn[j.Name]
							if assert.Truef(t, ok, "unable to find runsOn[%q] in test case", j.Name) {
								slices.Sort(j.RunsOn)
								slices.Sort(expected)
								assert.Equalf(t, expected, j.RunsOn, "comparing runsOn expectations for job %q", j.Name)
							}
						}
					}

					if tt.needs != nil {
						for _, j := range allJobsInRun {
							expected, ok := tt.needs[j.Name]
							if assert.Truef(t, ok, "unable to find needs[%q] in test case", j.Name) {
								slices.Sort(j.Needs)
								slices.Sort(expected)
								assert.Equalf(t, expected, j.Needs, "comparing needs expectations for job %q", j.Name)
							}
						}
					}

					if tt.expectIncompleteJob != nil {
						for _, j := range allJobsInRun {
							if slices.Contains(tt.expectIncompleteJob, j.Name) {
								m, _, err := j.HasIncompleteMatrix()
								require.NoError(t, err)
								r, _, _, err := j.HasIncompleteRunsOn()
								require.NoError(t, err)
								w, _, _, err := j.HasIncompleteWith()
								require.NoError(t, err)
								assert.True(t, m || r || w, "job %s was expected to still be marked as incomplete", j.Name)
							}
						}
					}

					if tt.localReusableWorkflowCallArgs != nil {
						require.Len(t, localReusableCalled, 1)
						assert.Equal(t, tt.localReusableWorkflowCallArgs, localReusableCalled[0])
						assert.Equal(t, 1, cleanupCallCount)
					}
				} else if tt.preExecutionError != 0 {
					// expectations are that the ActionRun has a populated PreExecutionError, is marked as failed
					actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: blockedJob.RunID})
					assert.Equal(t, tt.preExecutionError, actionRun.PreExecutionErrorCode)
					assert.Equal(t, tt.preExecutionErrorDetails, actionRun.PreExecutionErrorDetails)
					assert.Equal(t, actions_model.StatusFailure, actionRun.Status)

					// ActionRunJob is marked as failed
					blockedJobReloaded := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: tt.runJobID})
					assert.Equal(t, actions_model.StatusFailure, blockedJobReloaded.Status)

					// ensure all other jobs in this run are ignored
					assert.Equal(t, behaviourIgnoreAllJobsInRun, behaviour)
				} else {
					assert.Equal(t, behaviourExecuteJob, behaviour)
				}
			}

			if tt.notificationsVerifier != nil {
				tt.notificationsVerifier(notifier)
			}
		})
	}
}

func Test_tryHandleWorkflowCallOuterJob(t *testing.T) {
	tests := []struct {
		name            string
		runJobID        int64
		updateFields    []string
		outputs         map[string]string
		expectedAttempt int
	}{
		{
			name:     "not workflow call outer job",
			runJobID: 600,
		},
		{
			name:         "outputs for every context",
			runJobID:     601,
			updateFields: []string{"task_id", "attempt"},
			outputs: map[string]string{
				"from_inner_job":        "abcdefghijklmnopqrstuvwxyz",
				"from_inner_job_result": "success",
				"from_forgejo_ctx":      "refs/heads/main",
				"from_input_ctx":        "hello, world!",
				"from_vars_repo":        "this is a repo variable",
				"from_vars_org":         "this is an org variable",
				"from_vars_global":      "this is a global variable",
			},
			expectedAttempt: 1,
		},
		{
			name:         "attempt 2 rerun task",
			runJobID:     603,
			updateFields: []string{"task_id", "attempt"},
			outputs: map[string]string{
				"from_inner_job":        "abcdefghijklmnopqrstuvwxyz",
				"from_inner_job_result": "success",
				"from_forgejo_ctx":      "refs/heads/main",
				"from_input_ctx":        "hello, world!",
				"from_vars_repo":        "this is a repo variable",
				"from_vars_org":         "this is an org variable",
				"from_vars_global":      "this is a global variable",
			},
			expectedAttempt: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer unittest.OverrideFixtures("services/actions/Test_tryHandleWorkflowCallOuterJob")()
			require.NoError(t, unittest.PrepareTestDatabase())

			outerJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: tt.runJobID})
			require.EqualValues(t, 0, outerJob.TaskID)

			updateFields, err := tryHandleWorkflowCallOuterJob(t.Context(), outerJob)
			require.NoError(t, err)
			assert.Equal(t, tt.updateFields, updateFields)

			if tt.updateFields != nil {
				assert.EqualValues(t, tt.expectedAttempt, outerJob.Attempt)

				// TaskID expected to be set by tryHandleWorkflowCallOuterJob
				require.NotEqualValues(t, 0, outerJob.TaskID)

				taskOutputs, err := actions_model.FindTaskOutputByTaskID(t.Context(), outerJob.TaskID)
				require.NoError(t, err)
				outputMap := map[string]string{}
				for _, to := range taskOutputs {
					outputMap[to.OutputKey] = to.OutputValue
				}
				assert.Equal(t, tt.outputs, outputMap)
			}
		})
	}
}

func Test_checkJobsOfRun(t *testing.T) {
	defer unittest.OverrideFixtures("services/actions/Test_checkJobsOfRun")()
	require.NoError(t, unittest.PrepareTestDatabase())

	reusableWorkflow := `
on:
  workflow_call:
    inputs:
      argument:
        type: string

jobs:
  reusable:
    runs-on: ubuntu-latest
    steps:
      - run: |
          echo "Argument: ${{ inputs.argument }}"
`

	notifier := notify_service.NewMockNotifier(t)
	notifier.On("Run").Return().Maybe()
	notifier.On("NewWorkflowJobAttempt", mock.Anything, mock.Anything).Return(nil)
	notifier.On("WorkflowJobStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	notify_service.RegisterNotifier(notifier)
	defer notify_service.UnregisterNotifier(notifier)

	defer test.MockVariableValue(&lazyRepoExpandLocalReusableWorkflow,
		func(ctx context.Context, repoID int64, commitSHA string) (jobparser.LocalWorkflowFetcher, CleanupFunc) {
			fetcher := func(job *jobparser.Job, path string) ([]byte, error) {
				return []byte(reusableWorkflow), nil
			}
			cleanup := func() {
			}
			return fetcher, cleanup
		})()

	jobs, err := actions_model.GetRunJobsByRunID(t.Context(), 901)
	require.NoError(t, err)
	require.Len(t, jobs, 4)

	require.NoError(t, checkJobsOfRun(t.Context(), 901, 0))

	jobs, err = actions_model.GetRunJobsByRunID(t.Context(), 901)
	require.NoError(t, err)
	assert.Len(t, jobs, 5)

	assert.Equal(t, actions_model.JobIdentifier("a"), jobs[0].JobID)
	assert.Equal(t, actions_model.StatusSuccess.String(), jobs[0].Status.String())

	assert.Equal(t, actions_model.JobIdentifier("b"), jobs[1].JobID)
	assert.Equal(t, actions_model.StatusSuccess.String(), jobs[1].Status.String())

	assert.Equal(t, actions_model.JobIdentifier("reusable"), jobs[2].JobID)
	assert.Equal(t, actions_model.JobNamespace("namespace-b"), jobs[2].JobNamespace)
	assert.Equal(t, actions_model.StatusSuccess.String(), jobs[2].Status.String())

	assert.Equal(t, actions_model.JobIdentifier("c"), jobs[3].JobID)
	assert.Equal(t, actions_model.StatusBlocked.String(), jobs[3].Status.String())

	assert.Equal(t, actions_model.JobIdentifier("reusable"), jobs[4].JobID)
	assert.Equal(t, actions_model.StatusWaiting.String(), jobs[4].Status.String())

	notifier.AssertNumberOfCalls(t, "NewWorkflowJobAttempt", 2)
	notifier.AssertNumberOfCalls(t, "WorkflowJobStatusChanged", 1)
	notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 1)
	notifier.AssertCalled(
		t, "NewWorkflowJobAttempt", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == jobs[3].ID && job.JobID == actions_model.JobIdentifier("c") && job.Status == actions_model.StatusBlocked
		}),
	)
	notifier.AssertCalled(
		t, "NewWorkflowJobAttempt", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == jobs[4].ID && job.JobID == actions_model.JobIdentifier("reusable") && job.Status == actions_model.StatusBlocked
		}),
	)
	notifier.AssertCalled(
		t, "WorkflowJobStatusChanged", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == jobs[4].ID && job.JobID == actions_model.JobIdentifier("reusable") && job.Status == actions_model.StatusWaiting
		}),
		actions_model.StatusBlocked,
	)
	notifier.AssertCalled(
		t, "WorkflowJobCompleted", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == jobs[1].ID && job.JobID == actions_model.JobIdentifier("b") && job.Status == actions_model.StatusSuccess
		}),
		actions_model.StatusBlocked,
	)
}

func Test_checkJobsOfRun_ExpandsMatrixWithCorrectOutputJobStatuses(t *testing.T) {
	defer unittest.OverrideFixtures("services/actions/Test_checkJobsOfRun")()
	require.NoError(t, unittest.PrepareTestDatabase())

	notifier := notify_service.NewMockNotifier(t)
	notifier.On("Run").Return().Maybe()
	notifier.On("NewWorkflowJobAttempt", mock.Anything, mock.Anything).Return(nil)
	notifier.On("WorkflowJobStatusChanged", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	notifier.On("WorkflowRunEvent", mock.Anything, mock.Anything).Return(nil)

	notify_service.RegisterNotifier(notifier)
	defer notify_service.UnregisterNotifier(notifier)

	jobs, err := actions_model.GetRunJobsByRunID(t.Context(), 900)
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	require.NoError(t, checkJobsOfRun(t.Context(), 900, 0))

	jobs, err = actions_model.GetRunJobsByRunID(t.Context(), 900)
	require.NoError(t, err)
	assert.Len(t, jobs, 4)
	for _, job := range jobs {
		switch job.Name {
		case "define-matrix":
			assert.Equal(t, actions_model.StatusSuccess, job.Status)
		case "produce-artifacts (blue)":
			fallthrough
		case "produce-artifacts (green)":
			fallthrough
		case "produce-artifacts (red)":
			assert.Equal(t, actions_model.StatusWaiting, job.Status)
		default:
			assert.Fail(t, "unexpected job name")
		}
	}

	notifier.AssertNumberOfCalls(t, "NewWorkflowJobAttempt", 3)
	notifier.AssertNumberOfCalls(t, "WorkflowJobStatusChanged", 3)
	notifier.AssertNumberOfCalls(t, "WorkflowRunEvent", 1)
}
