// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package process

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
)

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

	// Go will invoke `Cancel` when the command's context is cancelled or deadline-exceeded; overwriting this method
	// implements the graceful shutdown.  Try to use a platform-specialized cancel routine, fallback to generic.
	gracefulCancel := platformSpecificGracefulCancel(cmd)
	if gracefulCancel == nil {
		gracefulCancel = genericGracefulCancel(cmd)
	}

	cmd.Cancel = gracefulCancel
}

// Check if an error from running a command is either context cancellation, or a process exit which appears to be caused
// by context cancellation which would be triggered by SetupCancellableCommand.
func IsErrCancellableCommandCancellation(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return true
	}

	// SetupCancellableCommand configures the command to SIGTERM on context cancellation, and then SIGKILL after a
	// graceful shutdown fails; detect both of these from the signal exit code.
	if exitError, isExitError := err.(*exec.ExitError); isExitError {
		if waitStatus, hasWaitStatus := exitError.Sys().(syscall.WaitStatus); hasWaitStatus {
			if waitStatus.Signaled() && (waitStatus.Signal() == syscall.SIGTERM || waitStatus.Signal() == syscall.SIGKILL) {
				return true
			}
		}
	}

	return false
}
