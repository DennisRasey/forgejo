// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetManager(t *testing.T) {
	go func() {
		// test race protection
		_ = GetManager()
	}()
	pm := GetManager()
	assert.NotNil(t, pm)
}

func TestManager_AddContext(t *testing.T) {
	pm := Manager{processMap: make(map[IDType]*process), next: 1}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p1Ctx, _, finished := pm.AddContext(ctx, "foo")
	defer finished()
	assert.NotEmpty(t, GetContext(p1Ctx).GetPID(), "expected to get non-empty pid")

	p2Ctx, _, finished := pm.AddContext(p1Ctx, "bar")
	defer finished()

	assert.NotEmpty(t, GetContext(p2Ctx).GetPID(), "expected to get non-empty pid")

	assert.NotEqual(t, GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetPID(), "expected to get different pids %s == %s", GetContext(p2Ctx).GetPID(), GetContext(p1Ctx).GetPID())
	assert.Equal(t, GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetParent().GetPID(), "expected to get pid %s got %s", GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetParent().GetPID())
}

func TestManager_Cancel(t *testing.T) {
	pm := Manager{processMap: make(map[IDType]*process), next: 1}

	ctx, _, finished := pm.AddContext(t.Context(), "foo")
	defer finished()

	pm.Cancel(GetPID(ctx))

	select {
	case <-ctx.Done():
	default:
		assert.FailNow(t, "Cancel should cancel the provided context")
	}
	finished()

	ctx, cancel, finished := pm.AddContext(t.Context(), "foo")
	defer finished()

	cancel()

	select {
	case <-ctx.Done():
	default:
		assert.FailNow(t, "Cancel should cancel the provided context")
	}
	finished()
}

func TestManager_Remove(t *testing.T) {
	pm := Manager{processMap: make(map[IDType]*process), next: 1}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	p1Ctx, _, finished := pm.AddContext(ctx, "foo")
	defer finished()
	assert.NotEmpty(t, GetContext(p1Ctx).GetPID(), "expected to have non-empty PID")

	p2Ctx, _, finished := pm.AddContext(p1Ctx, "bar")
	defer finished()

	assert.NotEqual(t, GetContext(p1Ctx).GetPID(), GetContext(p2Ctx).GetPID(), "expected to get different pids got %s == %s", GetContext(p2Ctx).GetPID(), GetContext(p1Ctx).GetPID())

	finished()

	_, exists := pm.processMap[GetPID(p2Ctx)]
	assert.False(t, exists, "PID %d is in the list but shouldn't", GetPID(p2Ctx))
}

func TestExecTimeoutNever(t *testing.T) {
	// TODO Investigate how to improve the time elapsed per round.
	maxLoops := 10
	for i := 1; i < maxLoops; i++ {
		_, stderr, err := GetManager().ExecTimeout(5*time.Second, "ExecTimeout", "git", "--version")
		if err != nil {
			t.Fatalf("git --version: %v(%s)", err, stderr)
		}
	}
}

func TestExecTimeoutAlways(t *testing.T) {
	maxLoops := 100
	for i := 1; i < maxLoops; i++ {
		_, stderr, err := GetManager().ExecTimeout(100*time.Microsecond, "ExecTimeout", "sleep", "5")
		// TODO Simplify logging and errors to get precise error type. E.g. checking "if err != context.DeadlineExceeded".
		if err == nil {
			t.Fatalf("sleep 5 secs: %v(%s)", err, stderr)
		}
	}
}

func TestSetupCancellableCommand(t *testing.T) {
	t.Run("context cancellation gives process time to clean itself up", func(t *testing.T) {
		script := `
		trap 'rm -f lock; exit 0' TERM
		touch lock
		touch created-lock
		while true;
		do
			sleep 0.05
		done
		`
		timeout := 15 * time.Second
		tmpdir := t.TempDir()

		childCtx, cancel := context.WithCancel(t.Context())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			_, _, err := GetManager().ExecDirEnvStdIn(childCtx, timeout, tmpdir, t.Name(), nil, nil, "bash", "-c", script)
			require.ErrorContains(t, err, "context canceled")
		})

		// 'created-lock' file should exist, indicating that the test script had created the lock file
		require.Eventually(t, func() bool {
			_, err := os.Stat(path.Join(tmpdir, "created-lock"))
			return err == nil
		}, time.Second, time.Millisecond)

		cancel()  // terminate the script by killing childCtx
		wg.Wait() // wait for Go to recognize process is killed

		// 'lock' file should not exist, as it should be cleaned up by the TERM trap
		_, err := os.Stat(path.Join(tmpdir, "lock"))
		require.Error(t, err, "lock file must not exist")
		assert.True(t, os.IsNotExist(err), "lock file must not exist")
	})

	t.Run("child processes are killed", func(t *testing.T) {
		// It's a bit tricky to observe the state of a grandchild process, especially when we're going to turn it into a
		// zombie.  os.FindProcess+p.Kill(0) are the documented way of checking whether a process is running, which we
		// could do based upon child-pid... but the process will be a zombie once its parent is killed without waiting
		// for it, and zombie processes absorb that kill with no response.
		//
		// The strategy used here is a tweak of forgejo-runner's `TestCancelLongRunningCommand` test, which faced the
		// same problem.  The subprocess writes a heartbeat file.  We delete it when we expect the process is killed,
		// and then verify it never reappears.
		script := `
		sh -c 'echo $$ > child-pid; while true; do touch child-heartbeat; sleep 0.05; done' &
		touch ready
		wait
		`
		timeout := 15 * time.Second
		tmpdir := t.TempDir()

		childCtx, cancel := context.WithCancel(t.Context())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			_, _, err := GetManager().ExecDirEnvStdIn(childCtx, timeout, tmpdir, t.Name(), nil, nil, "bash", "-c", script)
			require.ErrorContains(t, err, "context canceled")
		})

		// Wait for 'child-heartbeat' to exist indicating that the subprocess is running
		// childPidFile := path.Join(tmpdir, "child-pid")
		childHeartbeat := path.Join(tmpdir, "child-heartbeat")
		require.Eventually(t, func() bool {
			_, err := os.Stat(childHeartbeat)
			return err == nil
		}, time.Second, time.Millisecond)

		cancel()  // terminate the script by killing childCtx
		wg.Wait() // wait for Go to recognize process is killed

		require.NoError(t, os.Remove(childHeartbeat)) // remove heartbeat file

		// wait 2x the sleep period in the child process (50ms -> 100ms)
		time.Sleep(100 * time.Millisecond)

		// Verify that child heartbeat is not recreated, confirming the process was killed
		_, err := os.Stat(childHeartbeat)
		require.Error(t, err, "heartbeat file must not exist")
		assert.True(t, os.IsNotExist(err), "heartbeat file must not exist")
	})

	t.Run("fallback to SIGKILL", func(t *testing.T) {
		// cannot use test.MockVariableValue because it causes an import cycle, `process` is too low-level of a module;
		// t.Cleanup is used to restore the global values instead.
		originalTerminateGraceTimeout := TerminateGraceTimeout
		TerminateGraceTimeout = 100 * time.Millisecond

		originalNotifyTerminateGraceExhausted := NotifyTerminateGraceExhausted
		notifyOutput := []*exec.Cmd{}
		NotifyTerminateGraceExhausted = func(cmd *exec.Cmd) {
			notifyOutput = append(notifyOutput, cmd)
		}

		t.Cleanup(func() {
			TerminateGraceTimeout = originalTerminateGraceTimeout
			NotifyTerminateGraceExhausted = originalNotifyTerminateGraceExhausted
		})

		script := `
		trap 'touch term-step-1; sleep 30; touch term-step-2 exit 0' TERM
		touch script-running
		while true;
		do
			sleep 0.05
		done
		`

		timeout := 15 * time.Second
		tmpdir := t.TempDir()
		childCtx, cancel := context.WithCancel(t.Context())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			_, _, err := GetManager().ExecDirEnvStdIn(childCtx, timeout, tmpdir, t.Name(), nil, nil, "bash", "-c", script)
			require.ErrorContains(t, err, "context canceled")
		})

		// 'script-running' file should exist indicating that the trap on SIGTERM is set
		require.Eventually(t, func() bool {
			_, err := os.Stat(path.Join(tmpdir, "script-running"))
			return err == nil
		}, time.Second, time.Millisecond)

		beforeCancel := time.Now()
		assert.Empty(t, notifyOutput)
		cancel()  // terminate the script by killing childCtx
		wg.Wait() // wait for Go to recognize process is killed

		// Should take at least TerminateGraceTimeout to perform the cancel
		cancelDuration := time.Since(beforeCancel)
		assert.GreaterOrEqual(t, cancelDuration, TerminateGraceTimeout)

		// 'term-step-1' file should exist, indicating that SIGTERM was received
		_, err := os.Stat(path.Join(tmpdir, "term-step-1"))
		require.NoError(t, err)

		// 'term-step-2' file must not exist, as that would indicate that the script ran for the full "sleep 30" and wasn't SIGKILL'd
		_, err = os.Stat(path.Join(tmpdir, "term-step-2"))
		require.Error(t, err, "term-step-2 file must not exist")
		assert.True(t, os.IsNotExist(err), "term-step-2 file must not exist")

		// notifyOutput should contain the command that was run
		require.Len(t, notifyOutput, 1)
		cmdRun := notifyOutput[0]
		assert.Contains(t, cmdRun.Path, "bash")
	})
}

func TestIsCancellableCommandCancellation(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.False(t, IsErrCancellableCommandCancellation(nil))
	})

	t.Run("other", func(t *testing.T) {
		assert.False(t, IsErrCancellableCommandCancellation(errors.New("some other error")))
	})

	t.Run("SIGTERM", func(t *testing.T) {
		script := `
		kill -SIGTERM $$
		`
		cmd := exec.Command("bash", "-c", script)
		err := cmd.Run()
		assert.True(t, IsErrCancellableCommandCancellation(err))
	})

	t.Run("SIGKILL", func(t *testing.T) {
		script := `
		kill -SIGKILL $$
		`
		cmd := exec.Command("bash", "-c", script)
		err := cmd.Run()
		assert.True(t, IsErrCancellableCommandCancellation(err))
	})

	t.Run("ctx cancel", func(t *testing.T) {
		script := `
		sleep 30s
		`
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		cmd := exec.CommandContext(ctx, "bash", "-c", script)
		err := cmd.Run()
		assert.True(t, IsErrCancellableCommandCancellation(err))
	})
}
