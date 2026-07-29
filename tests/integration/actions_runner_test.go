// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/modules/setting"

	pingv1 "code.forgejo.org/forgejo/actions-proto/ping/v1"
	"code.forgejo.org/forgejo/actions-proto/ping/v1/pingv1connect"
	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"code.forgejo.org/forgejo/actions-proto/runner/v1/runnerv1connect"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockRunner struct {
	client           *mockRunnerClient
	uuid, token      string
	lastTasksVersion int64
}

type mockRunnerClient struct {
	pingServiceClient   pingv1connect.PingServiceClient
	runnerServiceClient runnerv1connect.RunnerServiceClient
}

func newMockRunner() *mockRunner {
	client := newMockRunnerClient("", "")
	return &mockRunner{client: client}
}

func newMockRunnerClientWithRequestKey(uuid, token, requestKey string) *mockRunnerClient {
	baseURL := fmt.Sprintf("%sapi/actions", setting.AppURL)

	opt := connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if uuid != "" {
				req.Header().Set("x-runner-uuid", uuid)
			}
			if token != "" {
				req.Header().Set("x-runner-token", token)
			}
			if requestKey != "" {
				req.Header().Set("x-runner-request-key", requestKey)
			}
			return next(ctx, req)
		}
	}))

	client := &mockRunnerClient{
		pingServiceClient:   pingv1connect.NewPingServiceClient(http.DefaultClient, baseURL, opt),
		runnerServiceClient: runnerv1connect.NewRunnerServiceClient(http.DefaultClient, baseURL, opt),
	}

	return client
}

func newMockRunnerClient(uuid, token string) *mockRunnerClient {
	return newMockRunnerClientWithRequestKey(uuid, token, "")
}

func (r *mockRunner) doPing(t *testing.T) {
	resp, err := r.client.pingServiceClient.Ping(t.Context(), connect.NewRequest(&pingv1.PingRequest{
		Data: "mock-runner",
	}))
	require.NoError(t, err)
	require.Equal(t, "Hello, mock-runner!", resp.Msg.Data)
}

func (r *mockRunner) doRegister(t *testing.T, name, token string, labels []string) {
	r.doPing(t)
	resp, err := r.client.runnerServiceClient.Register(t.Context(), connect.NewRequest(&runnerv1.RegisterRequest{
		Name:      name,
		Token:     token,
		Version:   "mock-runner-version",
		Labels:    labels,
		Ephemeral: false,
	}))
	require.NoError(t, err)
	r.uuid = resp.Msg.Runner.Uuid
	r.token = resp.Msg.Runner.Token
	r.client = newMockRunnerClient(r.uuid, r.token)
}

func (r *mockRunner) doRegisterEphemeral(t *testing.T, name, token string, labels []string) {
	r.doPing(t)
	resp, err := r.client.runnerServiceClient.Register(t.Context(), connect.NewRequest(&runnerv1.RegisterRequest{
		Name:      name,
		Token:     token,
		Version:   "mock-runner-version",
		Labels:    labels,
		Ephemeral: true,
	}))
	require.NoError(t, err)
	r.uuid = resp.Msg.Runner.Uuid
	r.token = resp.Msg.Runner.Token
	r.client = newMockRunnerClient(r.uuid, r.token)
}

func (r *mockRunner) setRequestKey(requestKey string) {
	r.client = newMockRunnerClientWithRequestKey(r.uuid, r.token, requestKey)
}

func (r *mockRunner) registerAsRepoRunner(t *testing.T, ownerName, repoName, runnerName string, labels []string) {
	if !setting.Database.Type.IsSQLite3() {
		assert.FailNow(t, "registering a mock runner when using a database other than SQLite leaves leftovers")
	}
	session := loginUser(t, ownerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runners/registration-token", ownerName, repoName)).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var registrationToken struct {
		Token string `json:"token"`
	}
	DecodeJSON(t, resp, &registrationToken)
	r.doRegister(t, runnerName, registrationToken.Token, labels)
}

func (r *mockRunner) registerAsEphemeralRepoRunner(t *testing.T, ownerName, repoName, runnerName string, labels []string) {
	if !setting.Database.Type.IsSQLite3() {
		assert.FailNow(t, "registering a mock runner when using a database other than SQLite leaves leftovers")
	}
	session := loginUser(t, ownerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runners/registration-token", ownerName, repoName)).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var registrationToken struct {
		Token string `json:"token"`
	}
	DecodeJSON(t, resp, &registrationToken)
	r.doRegisterEphemeral(t, runnerName, registrationToken.Token, labels)
}

func (r *mockRunner) fetchTaskOrError(t *testing.T, taskCapacity int64) (*runnerv1.Task, []*runnerv1.Task, error) {
	resp, err := r.client.runnerServiceClient.FetchTask(t.Context(), connect.NewRequest(&runnerv1.FetchTaskRequest{
		TasksVersion: r.lastTasksVersion,
		TaskCapacity: &taskCapacity,
	}))
	if err != nil {
		return nil, nil, err
	}
	r.lastTasksVersion = resp.Msg.TasksVersion
	return resp.Msg.Task, resp.Msg.AdditionalTasks, nil
}

func (r *mockRunner) maybeFetchTask(t *testing.T) *runnerv1.Task {
	resp, err := r.client.runnerServiceClient.FetchTask(t.Context(), connect.NewRequest(&runnerv1.FetchTaskRequest{
		TasksVersion: r.lastTasksVersion,
	}))
	require.NoError(t, err)
	r.lastTasksVersion = resp.Msg.TasksVersion
	return resp.Msg.Task
}

func (r *mockRunner) maybeFetchTaskWithTaskCapacity(t *testing.T, taskCapacity int64) (*runnerv1.Task, []*runnerv1.Task) {
	resp, err := r.client.runnerServiceClient.FetchTask(t.Context(), connect.NewRequest(&runnerv1.FetchTaskRequest{
		TasksVersion: r.lastTasksVersion,
		TaskCapacity: &taskCapacity,
	}))
	require.NoError(t, err)
	r.lastTasksVersion = resp.Msg.TasksVersion
	return resp.Msg.Task, resp.Msg.AdditionalTasks
}

func (r *mockRunner) maybeFetchSingleTask(t *testing.T, handle *string) *runnerv1.Task {
	resp, err := r.client.runnerServiceClient.FetchSingleTask(t.Context(), connect.NewRequest(&runnerv1.FetchSingleTaskRequest{
		TasksVersion: r.lastTasksVersion,
		Handle:       handle,
	}))
	require.NoError(t, err)
	r.lastTasksVersion = resp.Msg.TasksVersion
	return resp.Msg.Task
}

func (r *mockRunner) fetchTask(t *testing.T, timeout ...time.Duration) *runnerv1.Task {
	fetchTimeout := 10 * time.Second
	if len(timeout) > 0 {
		fetchTimeout = timeout[0]
	}

	var task *runnerv1.Task
	require.Eventually(t, func() bool {
		maybeTask := r.maybeFetchTask(t)
		if maybeTask != nil {
			task = maybeTask
			return true
		}
		return false
	}, fetchTimeout, time.Millisecond*100, "failed to fetch a task")
	return task
}

func (r *mockRunner) maybeFetchMultipleTasks(t *testing.T, taskCapacity *int64) (*runnerv1.Task, []*runnerv1.Task) {
	resp, err := r.client.runnerServiceClient.FetchTask(t.Context(), connect.NewRequest(&runnerv1.FetchTaskRequest{
		TasksVersion: r.lastTasksVersion,
		TaskCapacity: taskCapacity,
	}))
	require.NoError(t, err)
	r.lastTasksVersion = resp.Msg.TasksVersion
	return resp.Msg.Task, resp.Msg.AdditionalTasks
}

func (r *mockRunner) fetchMultipleTasks(t *testing.T, taskCapacity *int64, timeout ...time.Duration) (*runnerv1.Task, []*runnerv1.Task) {
	fetchTimeout := 10 * time.Second
	if len(timeout) > 0 {
		fetchTimeout = timeout[0]
	}
	var task *runnerv1.Task
	var additional []*runnerv1.Task
	require.Eventually(t, func() bool {
		maybeTask, maybeAdditional := r.maybeFetchMultipleTasks(t, taskCapacity)
		if maybeTask != nil {
			task = maybeTask
			additional = maybeAdditional
			return true
		}
		return false
	}, fetchTimeout, time.Millisecond*100, "failed to fetch a task")
	return task, additional
}

type mockTaskOutcome struct {
	result  runnerv1.Result
	outputs map[string]string
	logRows []*runnerv1.LogRow
	// stepStates, when non-nil, is included in the final UpdateTask's
	// TaskState.Steps. Lets tests exercise per-step LogIndex/LogLength
	// (and other StepState fields) without reaching into the DB directly.
	stepStates []*runnerv1.StepState
}

func (r *mockRunner) execTask(t *testing.T, task *runnerv1.Task, outcome *mockTaskOutcome) {
	for idx, lr := range outcome.logRows {
		resp, err := r.client.runnerServiceClient.UpdateLog(t.Context(), connect.NewRequest(&runnerv1.UpdateLogRequest{
			TaskId: task.Id,
			Index:  int64(idx),
			Rows:   []*runnerv1.LogRow{lr},
			NoMore: idx == len(outcome.logRows)-1,
		}))
		require.NoError(t, err)
		assert.EqualValues(t, idx+1, resp.Msg.AckIndex)
	}
	sentOutputKeys := make([]string, 0, len(outcome.outputs))
	for outputKey, outputValue := range outcome.outputs {
		resp, err := r.client.runnerServiceClient.UpdateTask(t.Context(), connect.NewRequest(&runnerv1.UpdateTaskRequest{
			State: &runnerv1.TaskState{
				Id:     task.Id,
				Result: runnerv1.Result_RESULT_UNSPECIFIED,
			},
			Outputs: map[string]string{outputKey: outputValue},
		}))
		require.NoError(t, err)
		sentOutputKeys = append(sentOutputKeys, outputKey)
		assert.ElementsMatch(t, sentOutputKeys, resp.Msg.SentOutputs)
	}
	resp, err := r.client.runnerServiceClient.UpdateTask(t.Context(), connect.NewRequest(&runnerv1.UpdateTaskRequest{
		State: &runnerv1.TaskState{
			Id:        task.Id,
			Result:    outcome.result,
			StoppedAt: timestamppb.Now(),
			Steps:     outcome.stepStates,
		},
	}))
	require.NoError(t, err)
	assert.Equal(t, outcome.result, resp.Msg.State.Result)
}

// Simply pretend we're running the task and succeed at that.
// We're that great!
func (r *mockRunner) succeedAtTask(t *testing.T, task *runnerv1.Task) {
	resp, err := r.client.runnerServiceClient.UpdateTask(t.Context(), connect.NewRequest(&runnerv1.UpdateTaskRequest{
		State: &runnerv1.TaskState{
			Id:        task.Id,
			Result:    runnerv1.Result_RESULT_SUCCESS,
			StoppedAt: timestamppb.Now(),
		},
	}))
	require.NoError(t, err)
	assert.Equal(t, runnerv1.Result_RESULT_SUCCESS, resp.Msg.State.Result)
}

// Pretend we're running the task, do nothing and fail at that.
func (r *mockRunner) failAtTask(t *testing.T, task *runnerv1.Task) {
	resp, err := r.client.runnerServiceClient.UpdateTask(t.Context(), connect.NewRequest(&runnerv1.UpdateTaskRequest{
		State: &runnerv1.TaskState{
			Id:        task.Id,
			Result:    runnerv1.Result_RESULT_FAILURE,
			StoppedAt: timestamppb.Now(),
		},
	}))
	require.NoError(t, err)
	assert.Equal(t, runnerv1.Result_RESULT_FAILURE, resp.Msg.State.Result)
}
