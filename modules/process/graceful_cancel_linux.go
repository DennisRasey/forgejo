// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package process

import (
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Platform-specific implementation of graceful cancellation.  After sending SIGTERM, uses a pidfd to monitor for
// process exit which isn't supported on all UNIXes.  pidfd is only supported on modern Linux (> 5.3), so a fallback to
// the generic routine is also present for Linux systems when that case is detected.
func platformSpecificGracefulCancel(cmd *exec.Cmd) func() error {
	return func() error {
		// We're going to TERM/KILL this process, but we cannot wait() on the PID within Cancel -- exactly one goroutine
		// may wait() the child process, and if we do it in here we'll cause random errors in the other goroutine.  A
		// PID FD is used to be able to get woken when the process dies, instead.
		//
		// It would be nice to use SysProcAttr to get the pidfd automatically, which would remove this syscall and
		// there'd be no chance of an error here.  But we wouldn't be able to close that FD if the process wasn't
		// cancelled since SetupCancellableCommand doesn't interact with commands that don't cancel.
		pidfd, err := unix.PidfdOpen(cmd.Process.Pid, 0)
		if isErrno(err, syscall.ENOSYS) {
			// pidfd not supported -- older than Linux 5.3 kernel.  Fallback to generic cancel routine.
			return genericGracefulCancel(cmd)()
		} else if isErrno(err, syscall.EINVAL) {
			// EINVAL -- pid is not valid, process already exited.  Not common since we haven't send SIGTERM yet, but
			// the process exit on its own before we cancel it.  Do nothing.
			return nil
		} else if killErrorSafeToIgnore(err) {
			return nil
		} else if err != nil {
			return err
		}
		defer unix.Close(pidfd)

		// Send SIGTERM to the entire process group by sending to the negative PID.
		err = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if killErrorSafeToIgnore(err) {
			return nil
		} else if err != nil {
			return err
		}

		// Begin waiting for the process to finish by polling the pidfd.  This is a bit ugly to have to implement here,
		// but as noted earlier we can't just `cmd.Process.Wait()` because another goroutine will be doing that
		// concurrently.  We need to repeatedly call Poll() until it indicates that the pidfd is readable, recalculating
		// remaining time in the grace period -- the repeated calls are needed because EINTR can interrupt the poll
		// briefly.
		pfds := []unix.PollFd{{Fd: int32(pidfd), Events: unix.POLLIN}}
		deadline := time.Now().Add(TerminateGraceTimeout)
		for {
			timeout := int(time.Until(deadline).Milliseconds())
			if timeout < 0 {
				// Grace period expired
				break
			}
			n, err := unix.Poll(pfds, timeout)
			if err == unix.EINTR {
				continue // interrupted by a signal; recompute remaining time and retry
			}
			if err != nil {
				return err
			}
			if n > 0 {
				// pidfd is readable, indicating that the process is terminated.  We don't bother actually reading the
				// exit code from the fd -- the other goroutine that is Wait()'ing will do that.
				return nil
			}
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
