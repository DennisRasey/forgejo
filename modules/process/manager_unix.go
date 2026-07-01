// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package process

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// A command run through [SetupCancellableCommand] will be sent SIGKILL after this grace period if the processes do not
// terminate after SIGTERM is sent to them.
var TerminateGraceTimeout = 5 * time.Second

// This closure is invoked in the event that SIGKILL is sent after a grace period has expired.  Intended for logging the
// situation, which cannot be performed inside the process module as it would cause a cyclical dependency on Forgejo's
// log module.
var NotifyTerminateGraceExhausted func(cmd *exec.Cmd)

// Identify errors that can occur in kill or wait that indicate the kill was actually successful as the process doesn't
// exist anymore.
func killErrorSafeToIgnore(err error) bool {
	errno, isErrno := err.(syscall.Errno)
	if isErrno {
		switch errno {
		case syscall.ESRCH, // No such process
			syscall.ECHILD: // No child processes
			return true
		}
	}
	syscallErr, isSyscallErr := err.(*os.SyscallError) // errno may be wrapped in SyscallError
	if isSyscallErr && killErrorSafeToIgnore(syscallErr.Err) {
		return true
	}
	return false
}

// Configures an [exec.Cmd] so that its Cancel function will gracefully shutdown the process and all subprocesses by
// sending them SIGTERM.  If they do not shutdown from the SIGTERM in the [TerminateGraceTimeout] grace period, they
// will be SIGKILL'd instead. Note that `SysProcAttr` and `Cancel` on the provided command are overwritten, and the
// command's `WaitDelay` will no longer have an impact.
func SetupCancellableCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// When running SubProcessA -> SubProcessB and SubProcessA gets killed by context timeout, use setpgid to make
		// sure the sub processes can be reaped instead of leaving defunct(zombie) processes.
		Setpgid: true,
	}

	// Override the default behaviour, which would have been to SIGKILL the process and wouldn't allow subprocesses the
	// ability to gracefully shutdown.  This could have lead to git subprocesses leaving lock files not cleaned up.
	cmd.Cancel = func() error {
		// We're going to TERM/KILL this process, but we cannot wait() on the PID within Cancel -- exactly one goroutine
		// may wait() the child process, and if we do it in here we'll cause random errors in the other goroutine.  A
		// PID FD is used to be able to get woken when the process dies, instead.
		//
		// It would be nice to use SysProcAttr to get the pidfd automatically, which would remove this syscall and
		// there'd be no chance of an error here.  But we wouldn't be able to close that FD if the process wasn't
		// cancelled since SetupCancellableCommand doesn't interact with commands that don't cancel.
		pidfd, err := unix.PidfdOpen(cmd.Process.Pid, 0)
		if killErrorSafeToIgnore(err) {
			return nil
		} else if err != nil {
			return err
		}
		defer unix.Close(pidfd)

		// When cmd is being killed (typically by Go when CommandContext's ctx is cancelled), send SIGTERM rather than
		// the default of SIGKILL.  Send it to the entire process group by sending to the negative PID.
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
