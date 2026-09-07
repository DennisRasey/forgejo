// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"strings"
	"testing"
	"unicode/utf8"

	"forgejo.org/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveTaskStepSummaries(t *testing.T) {
	t.Run("SaveAndUpdate", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		const taskID, repoID = int64(47), int64(4)
		stepOne := &ActionTaskStep{TaskID: taskID, Index: 0, RepoID: repoID}
		stepTwo := &ActionTaskStep{TaskID: taskID, Index: 1, RepoID: repoID}
		unittest.AssertSuccessfulInsert(t, stepOne, stepTwo)

		require.NoError(t, SaveTaskStepSummaries(t.Context(),
			&ActionTaskStepSummary{StepID: stepTwo.ID, TaskID: taskID, RepoID: repoID, Content: "## second"},
			&ActionTaskStepSummary{StepID: stepOne.ID, TaskID: taskID, RepoID: repoID, Content: "## first"},
		))
		summaries, err := GetTaskStepSummaries(t.Context(), taskID)
		require.NoError(t, err)
		require.Len(t, summaries, 2)
		// ordered by step regardless of the order they were saved in
		assert.Equal(t, stepOne.ID, summaries[0].StepID)
		assert.Equal(t, "## first", summaries[0].Content)
		assert.Equal(t, stepTwo.ID, summaries[1].StepID)
		assert.Equal(t, "## second", summaries[1].Content)

		// resending the full content of a step does an update instead of duplicating it
		require.NoError(t, SaveTaskStepSummaries(t.Context(),
			&ActionTaskStepSummary{StepID: stepOne.ID, TaskID: taskID, RepoID: repoID, Content: "### updated"},
		))
		summaries, err = GetTaskStepSummaries(t.Context(), taskID)
		require.NoError(t, err)
		require.Len(t, summaries, 2)
		assert.Equal(t, "### updated", summaries[0].Content)
		assert.Equal(t, "## second", summaries[1].Content)
	})

	t.Run("TruncatesOversizedContent", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		const taskID, repoID = int64(48), int64(4)
		step := &ActionTaskStep{TaskID: taskID, Index: 0, RepoID: repoID}
		unittest.AssertSuccessfulInsert(t, step)

		require.NoError(t, SaveTaskStepSummaries(t.Context(),
			&ActionTaskStepSummary{StepID: step.ID, TaskID: taskID, RepoID: repoID, Content: strings.Repeat("😁", MaxStepSummarySizeBytes+42)},
		))
		summaries, err := GetTaskStepSummaries(t.Context(), taskID)
		require.NoError(t, err)
		require.Len(t, summaries, 1)
		assert.LessOrEqual(t, len(summaries[0].Content), MaxStepSummarySizeBytes)
		assert.True(t, utf8.ValidString(summaries[0].Content))
		assert.True(t, strings.HasSuffix(summaries[0].Content, "…"))
	})
}

func TestDeleteTaskStepSummaries(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	const taskID, repoID = int64(49), int64(4)
	step := &ActionTaskStep{TaskID: taskID, Index: 0, RepoID: repoID}
	unittest.AssertSuccessfulInsert(t, step)
	require.NoError(t, SaveTaskStepSummaries(t.Context(),
		&ActionTaskStepSummary{StepID: step.ID, TaskID: taskID, RepoID: repoID, Content: "## gone"},
	))

	require.NoError(t, DeleteTaskStepSummaries(t.Context(), taskID))
	summaries, err := GetTaskStepSummaries(t.Context(), taskID)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}
