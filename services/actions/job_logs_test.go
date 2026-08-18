// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"bytes"
	"strings"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/actions"
	"forgejo.org/modules/json"
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

// TestWriteJobLogStream_TextNoFilter confirms the cheap path: when JSON is
// off, every line is passed through verbatim with a trailing newline.
func TestWriteJobLogStream_TextNoFilter(t *testing.T) {
	input := "2026-01-01T00:00:00.0000000Z hello\n2026-01-01T00:00:01.0000000Z world\n"
	var out bytes.Buffer
	require.NoError(t, WriteJobLogStream(&out, strings.NewReader(input), JobLogFilterOptions{}))
	// scanner.Bytes() doesn't include the newline; WriteJobLogStream re-adds it.
	assert.Equal(t, input, out.String())
}

// TestWriteJobLogStream_JSON confirms NDJSON output shape: one object per
// line with `time` and `content` fields, separated by `\n`.
func TestWriteJobLogStream_JSON(t *testing.T) {
	input := "2026-01-01T00:00:00.0000000Z hello\n2026-01-01T00:00:01.0000000Z world\n"
	var out bytes.Buffer
	require.NoError(t, WriteJobLogStream(&out, strings.NewReader(input), JobLogFilterOptions{JSON: true}))

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	require.Len(t, lines, 2)

	var l0 jsonLine
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &l0))
	assert.Equal(t, "hello", l0.Content)
	assert.False(t, l0.Time.IsZero())

	var l1 jsonLine
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &l1))
	assert.Equal(t, "world", l1.Content)
}

// TestWriteJobLogStream_SkipMalformed confirms the scanner tolerates lines
// that don't parse (no timestamp prefix) by skipping them rather than
// aborting the whole stream. Defense-in-depth — storage writes well-formed
// lines, so this shouldn't happen in practice.
func TestWriteJobLogStream_SkipMalformed(t *testing.T) {
	input := "2026-01-01T00:00:00.0000000Z hello\nbogus-line-no-prefix\n2026-01-01T00:00:02.0000000Z bye\n"
	var out bytes.Buffer
	require.NoError(t, WriteJobLogStream(&out, strings.NewReader(input), JobLogFilterOptions{JSON: true}))
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	assert.Len(t, lines, 2)
}

// TestMaxStoredLineSizeTracks sanity-checks that the exported constant the
// scanner uses tracks MaxLineSize (timestamp + space + content).
func TestMaxStoredLineSizeTracks(t *testing.T) {
	assert.Greater(t, actions.MaxStoredLineSize, actions.MaxLineSize)
}
