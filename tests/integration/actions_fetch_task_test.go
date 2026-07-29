// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/url"
	"strings"
	"sync"
	"testing"

	actions_model "forgejo.org/models/actions"
	unit_model "forgejo.org/models/unit"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/container"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/util"
	files_service "forgejo.org/services/repository/files"
	"forgejo.org/tests"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionFetchTask_TaskCapacity(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		// mock repo runner only supported on SQLite testing
		t.Skip()
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, _, f := tests.CreateDeclarativeRepo(t, user2, "repo-many-tasks",
			[]unit_model.Type{unit_model.TypeActions}, nil,
			[]*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".forgejo/workflows/matrix.yml",
					ContentReader: strings.NewReader(`
on:
  push:
jobs:
  job1:
    strategy:
      # matrix creates 125 different jobs from one push...
      matrix:
        d1: [a, b, c, d, e]
        d2: [a, b, c, d, e]
        d3: [a, b, c, d, e]
    runs-on: ubuntu-latest
    steps:
      - run: echo ${{ matrix.d1 }} ${{ matrix.d2 }} ${{ matrix.d3 }}
      - run: sleep 2
`),
				},
			},
		)
		defer f()

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		// Fetch with TaskCapacity undefined, set to nil, should return a single pending task
		task := runner.fetchTask(t)
		require.NotNil(t, task)
		assert.Contains(t, string(task.GetWorkflowPayload()), "name: job1 (a, a, a)")

		// After successfully fetching a task, the runner sets their next requested version to 0.  This allows it to
		// fetch back-to-back tasks without requiring that a server-side state change occurs.  That behaviour is
		// replicated here:
		runner.lastTasksVersion = 0

		// Fetch with TaskCapacity set to 1; additional should be nil
		capacity := int64(1)
		task, addt := runner.fetchMultipleTasks(t, &capacity)
		require.NotNil(t, task, "task")
		assert.Nil(t, addt, "addt")
		assert.Contains(t, string(task.GetWorkflowPayload()), "name: job1 (a, a, b)")

		runner.lastTasksVersion = 0

		capacity = 10
		task, addt = runner.fetchMultipleTasks(t, &capacity)
		require.NotNil(t, task, "task")
		require.NotNil(t, addt, "addt")
		assert.Contains(t, string(task.GetWorkflowPayload()), "name: job1 (a, a, c)")
		require.Len(t, addt, 9)
		assert.Contains(t, string(addt[0].GetWorkflowPayload()), "name: job1 (a, a, d)")
	})
}

func TestActionFetchTask_Idempotent(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		// mock repo runner only supported on SQLite testing
		t.Skip()
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, _, f := tests.CreateDeclarativeRepo(t, user2, "repo-many-tasks",
			[]unit_model.Type{unit_model.TypeActions}, nil,
			[]*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".forgejo/workflows/matrix.yml",
					ContentReader: strings.NewReader(`
on:
  push:
jobs:
  job1:
    strategy:
      matrix:
        d1: [a, b]
    runs-on: ubuntu-latest
    steps:
      - run: sleep 2
`),
				},
			},
		)
		defer f()

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		runner.setRequestKey("4b518ff2-00c6-4c22-ba05-77d5b597c2b4")

		// First request that fetches a task:
		task1 := runner.fetchTask(t)
		require.NotNil(t, task1)
		assert.Contains(t, string(task1.GetWorkflowPayload()), "name: job1")
		{
			// Base assumption, the FORGEJO_TOKEN secret can be identified... this is typical but we'll verify that it
			// doesn't work after the idempotent fetch.
			taskTokenTest, err := actions_model.GetRunningTaskByToken(t.Context(), task1.Secrets["FORGEJO_TOKEN"])
			require.NoError(t, err)
			assert.Equal(t, task1.Id, taskTokenTest.ID)
		}

		// Having retrieved a task... if we sent a fetchTask call with the same requestKey then we expect to get the
		// same task again:
		task1fetchedAgain := runner.fetchTask(t)
		require.NotNil(t, task1fetchedAgain)
		assert.Contains(t, string(task1fetchedAgain.GetWorkflowPayload()), "name: job1")

		assert.Equal(t, task1.Id, task1fetchedAgain.Id)
		assert.Equal(t, task1.WorkflowPayload, task1fetchedAgain.WorkflowPayload)
		m1 := task1.Context.AsMap()
		m1fetchedAgain := task1fetchedAgain.Context.AsMap()
		for k, v1 := range m1 {
			v2 := m1fetchedAgain[k]
			// "token" isn't expected to be the same as it is regenerated on recovery from idempotent fetch.  But it is
			// expected to be present, so we test for equal length.  "gitea_runtime_token" is a signed JWT which can
			// change between invocations based upon precise timestamps used, and so similarly should be validated to be
			// present not necessarily identical.
			if k == "token" || k == "gitea_runtime_token" || k == "forgejo_runtime_token" {
				assert.Len(t, v1.(string), len(v2.(string)))
			} else {
				assert.EqualValues(t, v1, v2, "context[%q]", k)
			}
		}
		for k, v1 := range task1.Secrets {
			v2 := task1fetchedAgain.Secrets[k]
			if k == "FORGEJO_TOKEN" || k == "GITEA_TOKEN" || k == "GITHUB_TOKEN" {
				// token isn't expected to be the same... but should be present.
				assert.Len(t, v1, len(v2))
			} else {
				assert.Equal(t, v1, v2, "secret[%q]", k)
			}
		}
		assert.Equal(t, task1.Needs, task1fetchedAgain.Needs)
		assert.Equal(t, task1.Vars, task1fetchedAgain.Vars)

		{
			// Original FORGEJO_TOKEN should not be usable anymore.
			_, err := actions_model.GetRunningTaskByToken(t.Context(), task1.Secrets["FORGEJO_TOKEN"])
			require.ErrorIs(t, err, util.ErrNotExist)
			// New FORGEJO_TOKEN should be usable.
			taskTokenTest, err := actions_model.GetRunningTaskByToken(t.Context(), task1fetchedAgain.Secrets["FORGEJO_TOKEN"])
			require.NoError(t, err)
			assert.Equal(t, task1fetchedAgain.Id, taskTokenTest.ID)
		}

		// But now if we change the request key, we don't expect to get the same task anymore:
		runner.setRequestKey("6d47d5f3-eaa2-449f-9040-8b20287401b3")
		task2 := runner.fetchTask(t)
		require.NotNil(t, task2)
		assert.NotEqual(t, task1.Id, task2.Id)
	})
}

func TestActionFetchTask_IdempotentConcurrent(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		// mock repo runner only supported on SQLite testing
		t.Skip()
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, _, f := tests.CreateDeclarativeRepo(t, user2, "repo-many-tasks",
			[]unit_model.Type{unit_model.TypeActions}, nil,
			[]*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".forgejo/workflows/matrix.yml",
					ContentReader: strings.NewReader(`
on:
  push:
jobs:
  job1:
    strategy:
      matrix:
        d1: [a, b, c, d, e]
        d2: [a, b, c, d, e]
    runs-on: ubuntu-latest
    steps:
      - run: sleep 2
`),
				},
			},
		)
		defer f()

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		runner.setRequestKey("c6dacc80-dace-4cea-9aad-f0e266355d8e")

		// If we make two simultaneous requests with the same runner request key, we should get either the error
		// "request key is currently locked; retry soon", or, the same tasks from both requests.
		concurrentCount := 15
		type fetchResult struct {
			index     int
			task      *runnerv1.Task
			addtTasks []*runnerv1.Task
			err       error
		}
		fetchResults := make(chan fetchResult, concurrentCount)

		var wg sync.WaitGroup
		for i := range concurrentCount {
			wg.Go(func() {
				// Larger task capacity is used to make the successful call take longer, cause higher chance of problems if
				// concurrency isn't handled correctly
				task, addtTasks, err := runner.fetchTaskOrError(t, 10)
				fetchResults <- fetchResult{index: i, task: task, addtTasks: addtTasks, err: err}
			})
		}

		wg.Wait()
		close(fetchResults)

		var firstResponseTaskIDs container.Set[int64]
		for res := range fetchResults {
			t.Logf("res = %#v", res)
			if res.task != nil {
				// This response had tasks, so let's ensure they're always the same for every response.
				taskIDs := container.Set[int64]{}
				taskIDs.Add(res.task.GetId())
				for _, extraTask := range res.addtTasks {
					assert.True(t, taskIDs.Add(extraTask.GetId()))
				}
				if firstResponseTaskIDs == nil {
					// first response with tasks -- record the IDs
					firstResponseTaskIDs = taskIDs
					assert.Len(t, taskIDs, 10)
				} else {
					// we've already found one response with tasks, so assert that they're all the same
					d1 := firstResponseTaskIDs.Difference(taskIDs)
					assert.Empty(t, d1, "first response taskIDs minus current response taskIDs should be empty")
					d2 := taskIDs.Difference(firstResponseTaskIDs)
					assert.Empty(t, d2, "current response taskIDs minus first response taskIDs should be empty")
				}
			} else if res.err != nil {
				require.ErrorContains(t, res.err, "request key is currently locked")
			} else {
				assert.Fail(t, "unexpected condition - res.task = nil, res.err = nil")
			}
		}

		assert.NotNil(t, firstResponseTaskIDs, "at least one response should return tasks")
	})
}

func TestActionFetchTask_RequestedJob(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		// mock repo runner only supported on SQLite testing
		t.Skip()
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// create the repo
		repo, _, f := tests.CreateDeclarativeRepo(t, user2, "repo-many-tasks",
			[]unit_model.Type{unit_model.TypeActions}, nil,
			[]*files_service.ChangeRepoFile{
				{
					Operation: "create",
					TreePath:  ".forgejo/workflows/simple.yml",
					ContentReader: strings.NewReader(`
on:
  push:
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo OK
  job2:
    runs-on: debian
    steps:
      - run: echo OK
  job3:
    runs-on: debian
    steps:
      - run: echo OK
`),
				},
			},
		)
		defer f()

		debianRunner := newMockRunner()
		debianRunner.registerAsRepoRunner(t, user2.Name, repo.Name, "debian-runner", []string{"debian"})

		ubuntuRunner := newMockRunner()
		ubuntuRunner.registerAsRepoRunner(t, user2.Name, repo.Name, "ubuntu-runner", []string{"ubuntu-latest"})

		job1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RepoID: repo.ID, Name: "job1"})
		job2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RepoID: repo.ID, Name: "job2"})
		job3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RepoID: repo.ID, Name: "job3"})

		assert.NotEmpty(t, job1.Handle)
		assert.NotEmpty(t, job2.Handle)
		assert.NotEmpty(t, job3.Handle)

		nonExistingHandle := "does-not-exist"
		emptyHandle := ""

		// The runner's labels do not match. Therefore, it does not receive the job despite explicitly asking for it.
		task := debianRunner.maybeFetchSingleTask(t, &job1.Handle)
		require.Nil(t, task)

		// If the requested job does not exist or is not ready, the runner does not receive any job.
		task = ubuntuRunner.maybeFetchSingleTask(t, &nonExistingHandle)
		require.Nil(t, task)

		ubuntuRunner.lastTasksVersion = 0
		debianRunner.lastTasksVersion = 0

		// The next job waiting in line for the debian-runner is job2. But because the runner explicitly asks for job3,
		// it receives job3 instead.
		task = debianRunner.maybeFetchSingleTask(t, &job3.Handle)
		require.NotNil(t, task)
		assert.Contains(t, string(task.GetWorkflowPayload()), "name: job3")

		ubuntuRunner.lastTasksVersion = 0
		debianRunner.lastTasksVersion = 0

		// Without explicitly asking for a job, the runners receives the next job waiting in line.
		task = debianRunner.maybeFetchSingleTask(t, nil)
		require.NotNil(t, task)
		assert.Contains(t, string(task.GetWorkflowPayload()), "name: job2")

		task = ubuntuRunner.maybeFetchSingleTask(t, &emptyHandle)
		require.NotNil(t, task)
		assert.Contains(t, string(task.GetWorkflowPayload()), "name: job1")
	})
}
