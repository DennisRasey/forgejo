// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package runner

import (
	"context"
	"strings"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/unittest"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateStepSummary(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
	firstStep := &actions_model.ActionTaskStep{TaskID: task.ID, Index: 0, RepoID: task.RepoID}
	secondStep := &actions_model.ActionTaskStep{TaskID: task.ID, Index: 1, RepoID: task.RepoID}
	unittest.AssertSuccessfulInsert(t, firstStep, secondStep)

	service := &Service{}
	ctxWithRunner := func(runnerID int64) context.Context {
		return context.WithValue(t.Context(), runnerCtxKey{}, &actions_model.ActionRunner{ID: runnerID})
	}

	updateStepSummary := func(runnerID int64, summaries ...*runnerv1.StepSummary) error {
		_, err := service.UpdateStepSummary(ctxWithRunner(runnerID), connect.NewRequest(&runnerv1.UpdateStepSummaryRequest{
			TaskId:    task.ID,
			Summaries: summaries,
		}))
		return err
	}

	t.Run("saving and updating the summaries", func(t *testing.T) {
		require.NoError(t, updateStepSummary(task.RunnerID,
			&runnerv1.StepSummary{StepNumber: 0, Content: "## first heading"},
			&runnerv1.StepSummary{StepNumber: 1, Content: "## second heading"},
		))
		summaries, err := actions_model.GetTaskStepSummaries(t.Context(), task.ID)
		require.NoError(t, err)
		require.Len(t, summaries, 2)
		assert.Equal(t, firstStep.ID, summaries[0].StepID)
		assert.Equal(t, "## first heading", summaries[0].Content)
		assert.Equal(t, secondStep.ID, summaries[1].StepID)
		assert.Equal(t, "## second heading", summaries[1].Content)

		// resending should only update it and not duplicate it
		require.NoError(t, updateStepSummary(task.RunnerID,
			&runnerv1.StepSummary{StepNumber: 0, Content: "### updated"},
		))
		summaries, err = actions_model.GetTaskStepSummaries(t.Context(), task.ID)
		require.NoError(t, err)
		require.Len(t, summaries, 2)
		assert.Equal(t, "### updated", summaries[0].Content)
	})

	t.Run("rejects invalid step numbers", func(t *testing.T) {
		err := updateStepSummary(task.RunnerID, &runnerv1.StepSummary{StepNumber: 42, Content: "## rejected"})
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("rejects invalid runner", func(t *testing.T) {
		err := updateStepSummary(task.RunnerID+1, &runnerv1.StepSummary{StepNumber: 0, Content: "## rejected"})
		require.Error(t, err)
	})

	t.Run("does nothing on empty summary", func(t *testing.T) {
		require.NoError(t, updateStepSummary(task.RunnerID))
	})

	t.Run("truncates if size exceeds MaxStepSummarySize", func(t *testing.T) {
		require.NoError(t, updateStepSummary(task.RunnerID,
			&runnerv1.StepSummary{StepNumber: 1, Content: strings.Repeat("x", actions_model.MaxStepSummarySizeBytes+42)},
		))
		summary := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTaskStepSummary{StepID: secondStep.ID})
		assert.LessOrEqual(t, len(summary.Content), actions_model.MaxStepSummarySizeBytes)
	})
}
