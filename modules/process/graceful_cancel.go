// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package process

import (
	"os"
	"os/exec"
	"slices"
	"syscall"
	"time"
)

// Identify errors that can occur in kill or wait that indicate the kill was actually successful as the process doesn't
// exist anymore.
func killErrorSafeToIgnore(err error) bool {
	return isErrno(err,
		syscall.ESRCH,  // No such process
		syscall.ECHILD, // No child processes
	)
}

func isErrno(err error, errnoList ...syscall.Errno) bool {
	errno, is := err.(syscall.Errno)
	if is {
		if slices.Contains(errnoList, errno) {
			return true
		}
	}
	syscallErr, isSyscallErr := err.(*os.SyscallError) // errno may be wrapped in SyscallError
	if isSyscallErr && isErrno(syscallErr.Err, errnoList...) {
		return true
	}
	return false
}

// Create an implementation of `exec.Cmd.Cancel` which gracefully shuts down the process, sending SIGTERM, waiting a
// grace period, and sending SIGKILL.
func genericGracefulCancel(cmd *exec.Cmd) func() error {
	return func() error {
		// Send SIGTERM to the entire process group by sending to the negative PID.
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if killErrorSafeToIgnore(err) {
			return nil
		} else if err != nil {
			return err
		}

		// We can't just `cmd.Process.Wait()` because another goroutine will be doing that concurrently, but
		// conceptually that is what we want to do.  This approach sends Signal(0) to the process until that returns an
		// error (indicating that the process is terminated) or until the grace interval is complete, with a delay
		// between loops.
		//
		// The arbitrary delay between loops isn't ideal when compared to letting the OS wake us when the process is
		// done (as the Linux pidfd implementation does), but it's cross-platform.  As this is already an edge-case
		// where a subprocess has an abnormal termination caused by Forgejo, which isn't very frequent, a loop and wait
		// implementation shouldn't have significant performance impact.
		deadline := time.Now().Add(TerminateGraceTimeout)
		for time.Until(deadline) >= 0 {
			err := cmd.Process.Signal(syscall.Signal(0))
			if err == os.ErrProcessDone {
				// Process has terminated.
				return nil
			} else if err != nil {
				return err
			}
			// Don't busy-loop -- take a small breather to avoid 100% CPU usage.
			time.Sleep(50 * time.Millisecond)
		}

		// After sending SIGTERM, we hit the terminate grace timeout and the subprocess is still running.  Send SIGKILL
		// to the process group as a fallback.
		if NotifyTerminateGraceExhausted != nil {
			NotifyTerminateGraceExhausted(cmd)
		}
		err = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if killErrorSafeToIgnore(err) {
			return nil
		}
		return err
	}
}
