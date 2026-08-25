// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later
package unittest

import (
	"context"
	"errors"

	"code.forgejo.org/xorm/xorm/contexts"
)

var (
	faultInjectorCount      int64
	faultInjectorNumQueries int64 = -1
	ErrFaultInjected              = errors.New("nobody expects a fault injection")
)

type faultInjectorHook struct{}

var _ contexts.Hook = &faultInjectorHook{}

func (faultInjectorHook) BeforeProcess(c *contexts.ContextHook) (context.Context, error) {
	if faultInjectorNumQueries == -1 {
		return c.Ctx, nil
	}

	// Always allow ROLLBACK, we always want to allow for transactions to get cancelled.
	if faultInjectorCount == faultInjectorNumQueries && c.SQL != "ROLLBACK" {
		return c.Ctx, ErrFaultInjected
	}

	faultInjectorCount++

	return c.Ctx, nil
}

func (faultInjectorHook) AfterProcess(*contexts.ContextHook) error {
	return nil
}

// Allow `numQueries` before all database queries will fail until the
// returning function is executed.
func SetFaultInjector(numQueries int64) func() {
	faultInjectorNumQueries = numQueries

	return func() {
		faultInjectorNumQueries = -1
		faultInjectorCount = 0
	}
}
