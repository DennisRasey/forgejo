// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later
package unittest

import (
	"testing"

	"code.forgejo.org/xorm/xorm/contexts"
	"github.com/stretchr/testify/require"
)

func TestFaultInjector(t *testing.T) {
	faultInjector := faultInjectorHook{}
	c := &contexts.ContextHook{
		Ctx: t.Context(),
		SQL: "Hello, 世界", // We don't check for valid SQL anyway.
	}

	t.Run("Should not block", func(t *testing.T) {
		// Currently no fault injection is set, so this should go through.
		for range 100 {
			_, err := faultInjector.BeforeProcess(c)
			require.NoError(t, err)
		}
	})

	t.Run("Reset", func(t *testing.T) {
		// Okay only allow one query to go through.
		reset := SetFaultInjector(1)

		// Do the only query.
		_, err := faultInjector.BeforeProcess(c)
		require.NoError(t, err)

		// Now we reset, we don't check the blocking behavior yet. We first
		// must know that we can safely reset.
		reset()

		// This should go through.
		for range 100 {
			_, err := faultInjector.BeforeProcess(c)
			require.NoError(t, err)
		}
	})

	t.Run("Blocking", func(t *testing.T) {
		// Okay only allow one query to go through.
		reset := SetFaultInjector(1)

		// Do the only query.
		_, err := faultInjector.BeforeProcess(c)
		require.NoError(t, err)

		// Any query now will return a error.
		for range 100 {
			_, err := faultInjector.BeforeProcess(c)
			require.ErrorIs(t, err, ErrFaultInjected)
		}

		// Ah but there's a exemption for `ROLLBACK`.
		_, err = faultInjector.BeforeProcess(&contexts.ContextHook{Ctx: t.Context(), SQL: "ROLLBACK"})
		require.NoError(t, err)

		reset()
	})

	t.Run("Number of queries", func(t *testing.T) {
		// For funsies lets test a bunch of max numbers of queries.
		for i := range int64(1024) {
			// Allow i queries
			reset := SetFaultInjector(i)

			// Make i queries.
			for range i {
				_, err := faultInjector.BeforeProcess(c)
				require.NoError(t, err)
			}

			// After i'th query it returns a error.
			_, err := faultInjector.BeforeProcess(c)
			require.ErrorIs(t, err, ErrFaultInjected)

			reset()
		}
	})
}
