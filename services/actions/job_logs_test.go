// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover the error paths of OpenJobLogReader. Each one terminates
// before actions.OpenLogs is called, so the tests don't need real log files
// in DBFS — LogFilename can point at "does-not-exist".

func TestOpenJobLogReader_RepoMismatch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertSuccessfulInsert(t, &actions_model.ActionRunJob{ID: 9001, RepoID: 1, TaskID: 9001})

	otherRepo := &repo_model.Repository{ID: 2}
	_, _, _, err := OpenJobLogReader(db.DefaultContext, otherRepo, 9001, optional.None[int64](), optional.None[int]())
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestOpenJobLogReader_JobNotExecuted(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertSuccessfulInsert(t, &actions_model.ActionRunJob{ID: 9002, RepoID: 1, TaskID: 0})

	repo := &repo_model.Repository{ID: 1}
	_, _, _, err := OpenJobLogReader(db.DefaultContext, repo, 9002, optional.None[int64](), optional.None[int]())
	assert.ErrorIs(t, err, ErrJobNotExecuted)
}

func TestOpenJobLogReader_LogsExpired(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertSuccessfulInsert(t, &actions_model.ActionTask{ID: 9003, LogExpired: true})
	unittest.AssertSuccessfulInsert(t, &actions_model.ActionRunJob{ID: 9003, RepoID: 1, TaskID: 9003})

	repo := &repo_model.Repository{ID: 1}
	_, _, _, err := OpenJobLogReader(db.DefaultContext, repo, 9003, optional.None[int64](), optional.None[int]())
	assert.ErrorIs(t, err, ErrLogsExpired)
}

func TestOpenJobLogReader_UnknownAttempt(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertSuccessfulInsert(t, &actions_model.ActionTask{ID: 9004, JobID: 9004, Attempt: 1})
	unittest.AssertSuccessfulInsert(t, &actions_model.ActionRunJob{ID: 9004, RepoID: 1, TaskID: 9004})

	repo := &repo_model.Repository{ID: 1}
	_, _, _, err := OpenJobLogReader(db.DefaultContext, repo, 9004, optional.Some(int64(999)), optional.None[int]())
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func TestOpenJobLogReader_StepOutOfRange(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Task with one real step → FullSteps returns 3 entries (setup, real, complete).
	// step=99 is out of range.
	unittest.AssertSuccessfulInsert(t, &actions_model.ActionTask{
		ID:          9005,
		LogFilename: "does-not-exist",
		LogIndexes:  []int64{0},
		LogLength:   1,
		LogSize:     100,
	})
	unittest.AssertSuccessfulInsert(t, &actions_model.ActionRunJob{ID: 9005, RepoID: 1, TaskID: 9005})
	unittest.AssertSuccessfulInsert(t, &actions_model.ActionTaskStep{
		ID:        9005,
		TaskID:    9005,
		Index:     0,
		LogIndex:  0,
		LogLength: 1,
	})

	repo := &repo_model.Repository{ID: 1}
	_, _, _, err := OpenJobLogReader(db.DefaultContext, repo, 9005, optional.None[int64](), optional.Some(99))
	assert.ErrorIs(t, err, ErrStepOutOfRange)
}
