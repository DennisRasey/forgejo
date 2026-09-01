// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"errors"
	"sync"
	"testing"
	"time"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	"forgejo.org/services/actions"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTaskForRunnerNoMatchingJobFound(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestCreateTaskForRunner")()
	defer tests.PrepareTestEnv(t)()

	runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: 1004})

	_, err := actions.CreateTaskForRunner(t.Context(), runner, nil, nil)
	require.ErrorIs(t, err, actions_model.ErrNoMatchingJobFound)
}

// We need to concurrently choose the same job with two requests to CreateTaskForRunner.  The second
// request that tries to update the job in the database (that was already updated by the first
// request) then chokes and returns the error ErrNoJobUpdated.
func TestCreateTaskForRunnerNoJobUpdated(t *testing.T) {
	if setting.Database.Type.IsSQLite3() {
		// SQLite locks on the transaction and the desired race condition can't be achieved
		t.Skip()
	}

	defer unittest.OverrideFixtures("tests/integration/fixtures/TestCreateTaskForRunner")()
	defer tests.PrepareTestEnv(t)()

	assert.Eventually(
		t,
		func() bool {
			unittest.LoadFixtures()

			runner := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{ID: 1005})

			var w sync.WaitGroup
			errs := make(chan error, 2)
			for range 2 {
				w.Go(func() {
					_, err := actions.CreateTaskForRunner(t.Context(), runner, nil, nil)
					errs <- err
				})
			}
			w.Wait()
			close(errs)

			gotNoJobUpdated := false
			succeded := false
			for err := range errs {
				if errors.Is(err, actions_model.ErrNoJobUpdated) {
					gotNoJobUpdated = true
				}
				if err == nil {
					succeded = true
				}
			}

			return succeded && gotNoJobUpdated
		},
		30*time.Second,
		500*time.Microsecond,
		"one call to CreateTaskForRunner should succeed and the other one should return the error ErrNoJobUpdated",
	)
}
