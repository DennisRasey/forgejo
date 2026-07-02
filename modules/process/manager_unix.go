// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package process

import (
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
