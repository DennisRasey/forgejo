// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
	"forgejo.org/modules/log"
	"forgejo.org/modules/util"
)

// MaxStepSummarySizeBytes is the maximum size in bytes of the summary content of a single step.
// The value of 1MiB mirrors the per-step limit GitHub enforces on GITHUB_STEP_SUMMARY as of August 2026
// and matches the size at which the runner truncates a step summary before uploading it,
// making this cap apply for other consumers.
// Lowering this value poses the risk of compatibility problems - as in composite actions that directly append to the summary -
// while increasing this value should be fine.
// If this value is altered it must be updated in the runner (act/runner/step.go) as well and a new runner has to be released.
const MaxStepSummarySizeBytes = 1024 * 1024

// ActionTaskStepSummary holds the GITHUB_STEP_SUMMARY markdown produced by a single step of an ActionTask.
type ActionTaskStepSummary struct {
	ID      int64  `xorm:"pk autoincr"`
	StepID  int64  `xorm:"UNIQUE NOT NULL REFERENCES(action_task_step, id)"`
	TaskID  int64  `xorm:"INDEX NOT NULL REFERENCES(action_task, id)"`
	RepoID  int64  `xorm:"INDEX NOT NULL REFERENCES(repository, id)"`
	Content string `xorm:"MEDIUMTEXT NOT NULL"`
}

func init() {
	db.RegisterModel(new(ActionTaskStepSummary))
}

// GetTaskStepSummaries returns the step summaries of the task.
func GetTaskStepSummaries(ctx context.Context, taskID int64) ([]*ActionTaskStepSummary, error) {
	var summaries []*ActionTaskStepSummary
	return summaries, db.GetEngine(ctx).Where("task_id = ?", taskID).OrderBy("step_id ASC").Find(&summaries)
}

// GetTaskStepSummariesByStepID returns the step summaries of the task, keyed by their StepID.
func GetTaskStepSummariesByStepID(ctx context.Context, taskID int64) (map[int64]*ActionTaskStepSummary, error) {
	summaries, err := GetTaskStepSummaries(ctx, taskID)
	if err != nil {
		return nil, err
	}
	summariesByStepID := make(map[int64]*ActionTaskStepSummary, len(summaries))
	for _, summary := range summaries {
		summariesByStepID[summary.StepID] = summary
	}
	return summariesByStepID, nil
}

// SaveTaskStepSummaries upserts the given step summaries by their StepID in a single transaction.
// Contents beyond MaxStepSummarySize are truncated.
func SaveTaskStepSummaries(ctx context.Context, summaries ...*ActionTaskStepSummary) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		e := db.GetEngine(ctx)
		for _, summary := range summaries {
			if len(summary.Content) > MaxStepSummarySizeBytes {
				log.Debug("Summary of step %d of task %d exceeds the size limit of %d bytes and is truncated, (the runner should have truncated it already tho)",
					summary.StepID, summary.TaskID, MaxStepSummarySizeBytes)
				summary.Content = util.TruncateStringAtWordBoundary(summary.Content, MaxStepSummarySizeBytes)
			}
			existing := &ActionTaskStepSummary{}
			has, err := e.Where("step_id = ?", summary.StepID).Get(existing)
			if err != nil {
				return fmt.Errorf("failed to get the existing summary of step %d: %w", summary.StepID, err)
			}
			if has {
				summary.ID = existing.ID
				if _, err := e.ID(existing.ID).Cols("content").Update(summary); err != nil {
					return fmt.Errorf("failed to update the summary of step %d: %w", summary.StepID, err)
				}
			} else if _, err := e.Insert(summary); err != nil {
				return fmt.Errorf("failed to insert the summary of step %d: %w", summary.StepID, err)
			}
		}
		return nil
	})
}

// DeleteTaskStepSummaries removes the step summaries of the task. Due to the foreign key on StepID it has to run
// before the steps of the task are deleted.
func DeleteTaskStepSummaries(ctx context.Context, taskID int64) error {
	if _, err := db.GetEngine(ctx).Delete(&ActionTaskStepSummary{TaskID: taskID}); err != nil {
		return fmt.Errorf("failed to delete the step summaries of task %d: %w", taskID, err)
	}
	return nil
}
