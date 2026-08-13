// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/url"
	"strings"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	unit_model "forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	actions_service "forgejo.org/services/actions"
	files_service "forgejo.org/services/repository/files"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionConcurrencyRunnerFiltering(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionConcurrencyRunnerFiltering")()
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, tc := range []struct {
		name           string
		runnerName     string
		expectedRunIDs []int64
	}{
		{
			// owner id 2
			runnerName:     "User runner",
			expectedRunIDs: []int64{500, 502},
		},
		{
			// owner id 3
			runnerName:     "Organisation runner",
			expectedRunIDs: []int64{501},
		},
		{
			runnerName:     "Repository runner",
			expectedRunIDs: []int64{502},
		},
		{
			runnerName:     "Global runner",
			expectedRunIDs: []int64{500, 501, 502},
		},
	} {
		t.Run(tc.runnerName, func(t *testing.T) {
			doTest := func(t *testing.T) {
				t.Helper()
				e := db.GetEngine(t.Context())

				runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{Name: tc.runnerName})
				jobs, err := actions_model.GetAvailableJobsForRunner(e, runner)
				require.NoError(t, err)

				ids := []int64{}
				for _, job := range jobs {
					ids = append(ids, job.ID)
				}
				assert.ElementsMatch(t, tc.expectedRunIDs, ids)
			}

			t.Run("ConcurrencyGroupQueueEnabled", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				defer test.MockVariableValue(&setting.Actions.ConcurrencyGroupQueueEnabled, true)()
				doTest(t)
			})

			t.Run("ConcurrencyGroupQueueDisabled", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				defer test.MockVariableValue(&setting.Actions.ConcurrencyGroupQueueEnabled, false)()
				doTest(t)
			})
		})
	}
}

// These tests are a little more unit-testy than they are integration tests, but they're placed in the integration test
// suite so that they're run on all database engines.
func TestActionConcurrencyGroupQueue(t *testing.T) {
	for _, tc := range []struct {
		name            string
		expectedRunIDs  []int64
		updateRun500    map[string]any
		updateRunJob500 map[string]any
		updateRun501    map[string]any
		updateRunJob501 map[string]any
		queuingDisabled bool
	}{
		{
			name:            "queuing disabled",
			expectedRunIDs:  []int64{500, 501, 502},
			queuingDisabled: true,
		},
		{
			// Job 501 & 502's data is configured to be queued-behind job 500, so with queuing enabled it shouldn't
			// appear.
			name:           "concurrency blocked",
			expectedRunIDs: []int64{500},
		},
		{
			name:           "different repo",
			updateRun501:   map[string]any{"repo_id": 2},
			expectedRunIDs: []int64{500, 501},
		},
		{
			name:           "different concurrency group",
			updateRun501:   map[string]any{"concurrency_group": "321bca"},
			expectedRunIDs: []int64{500, 501},
		},
		{
			name:           "null concurrency group",
			updateRun501:   map[string]any{"concurrency_group": nil},
			expectedRunIDs: []int64{500, 501},
		},
		{
			name:           "empty concurrency group",
			updateRun501:   map[string]any{"concurrency_group": ""},
			expectedRunIDs: []int64{500, 501},
		},
		{
			name:           "unlimited concurrency",
			updateRun501:   map[string]any{"concurrency_type": actions_model.UnlimitedConcurrency},
			expectedRunIDs: []int64{500, 501},
		},
		{
			name:           "cancel-in-progress type",
			updateRun501:   map[string]any{"concurrency_type": actions_model.CancelInProgress},
			expectedRunIDs: []int64{500, 501},
		},
		{
			name:            "blocking job done",
			updateRun500:    map[string]any{"status": actions_model.StatusCancelled},
			updateRunJob500: map[string]any{"status": actions_model.StatusCancelled},
			expectedRunIDs:  []int64{501},
		},
		{
			name:            "mid-index job running",
			updateRun501:    map[string]any{"status": actions_model.StatusRunning},
			updateRunJob501: map[string]any{"status": actions_model.StatusRunning},
			expectedRunIDs:  []int64{},
		},
		{
			// Reflects a case where 500 may be retried -- there's already a later job (index-wise) in the concurrency
			// group that is done, but if 500 is waiting it can still be run
			name:            "mid-index job ran",
			updateRun501:    map[string]any{"status": actions_model.StatusSuccess},
			updateRunJob501: map[string]any{"status": actions_model.StatusSuccess},
			expectedRunIDs:  []int64{500},
		},
		{
			// If both job 500 & job 501 are in the same workflow run, and one is running, the other can still start
			// (this would be conditional on its `needs` as a job, but that isn't evaluated by GetAvailableJobsForRunner
			// so isn't in the scope of testing here)
			name:            "multiple jobs from same run",
			updateRun500:    map[string]any{"status": actions_model.StatusRunning},
			updateRunJob500: map[string]any{"status": actions_model.StatusRunning},
			updateRunJob501: map[string]any{"run_id": 500},
			expectedRunIDs:  []int64{501},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionConcurrencyGroupQueue")()
			require.NoError(t, unittest.PrepareTestDatabase())
			runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: 1004}, "owner_id = 0 AND repo_id = 0")

			defer test.MockVariableValue(&setting.Actions.ConcurrencyGroupQueueEnabled, !tc.queuingDisabled)()

			e := db.GetEngine(t.Context())

			if tc.updateRun500 != nil {
				affected, err := e.Table(&actions_model.ActionRun{}).Where("id = ?", 500).Update(tc.updateRun500)
				require.NoError(t, err)
				require.EqualValues(t, 1, affected)
			}
			if tc.updateRunJob500 != nil {
				affected, err := e.Table(&actions_model.ActionRunJob{}).Where("id = ?", 500).Update(tc.updateRunJob500)
				require.NoError(t, err)
				require.EqualValues(t, 1, affected)
			}
			if tc.updateRun501 != nil {
				affected, err := e.Table(&actions_model.ActionRun{}).Where("id = ?", 501).Update(tc.updateRun501)
				require.NoError(t, err)
				require.EqualValues(t, 1, affected)
			}
			if tc.updateRunJob501 != nil {
				affected, err := e.Table(&actions_model.ActionRunJob{}).Where("id = ?", 501).Update(tc.updateRunJob501)
				require.NoError(t, err)
				require.EqualValues(t, 1, affected)
			}

			jobs, err := actions_model.GetAvailableJobsForRunner(e, runner)
			require.NoError(t, err)

			ids := []int64{}
			for _, job := range jobs {
				ids = append(ids, job.ID)
			}
			assert.ElementsMatch(t, tc.expectedRunIDs, ids)
		})
	}
}

func TestActionConcurrencyGroupQueueFetchNext(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		// mock repo runner only supported on SQLite testing
		t.Skip()
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, sha, f := tests.CreateDeclarativeRepo(t, user2, "repo-workflow-dispatch",
			[]unit_model.Type{unit_model.TypeActions}, nil,
			[]*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".forgejo/workflows/dispatch.yml",
					ContentReader: strings.NewReader(
						"name: concurrency group workflow\n" +
							"on:\n" +
							"  workflow_dispatch:\n" +
							"    inputs:\n" +
							"      ident:\n" +
							"        type: string\n" +
							"concurrency:\n" +
							"  group: abc\n" +
							"  cancel-in-progress: false\n" +
							"jobs:\n" +
							"  test:\n" +
							"    runs-on: ubuntu-latest\n" +
							"    steps:\n" +
							"      - run: echo deployment goes here\n",
					),
				},
			},
		)
		defer f()

		gitRepo, err := gitrepo.OpenRepository(db.DefaultContext, repo)
		require.NoError(t, err)
		defer gitRepo.Close()

		workflow, err := actions_service.GetWorkflowFromCommit(gitRepo, "main", "dispatch.yml")
		require.NoError(t, err)
		assert.Equal(t, "refs/heads/main", workflow.Ref)
		assert.Equal(t, sha, workflow.Commit.ID.String())

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		// first run within the concurrency group
		_, _, err = workflow.Dispatch(db.DefaultContext, func(key string) string { return "task1" }, repo, user2)
		require.NoError(t, err)
		task1 := runner.fetchTask(t)

		// dispatch a second run within the same concurrency group
		_, _, err = workflow.Dispatch(db.DefaultContext, func(key string) string { return "task2" }, repo, user2)
		require.NoError(t, err)

		// assert that we can't fetch and start that second task -- it's blocked behind the first
		task2 := runner.maybeFetchTask(t)
		assert.Nil(t, task2)

		// finish the first task
		runner.succeedAtTask(t, task1)

		// now task2 should be accessible since task1 has completed
		task2 = runner.fetchTask(t)
		assert.NotNil(t, task2)
		runner.succeedAtTask(t, task2)
	})
}
