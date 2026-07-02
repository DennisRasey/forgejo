// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

// By default _unix is built on Linux as well, but we have a _linux specialization using pidfd.
//go:build !linux

package process

import (
	"os/exec"
)

func platformSpecificGracefulCancel(cmd *exec.Cmd) func() error {
	return nil
}
