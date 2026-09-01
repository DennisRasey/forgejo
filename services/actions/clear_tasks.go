// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	"forgejo.org/modules/actions"
	"forgejo.org/modules/log"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"
)

// StopZombieTasks stops the task which have running status, but haven't been updated for a long time
func StopZombieTasks(ctx context.Context) error {
	return stopTasks(ctx, actions_model.FindTaskOptions{
		Status:        []actions_model.Status{actions_model.StatusRunning},
		UpdatedBefore: timeutil.TimeStamp(time.Now().Add(-setting.Actions.ZombieTaskTimeout).Unix()),
	})
}

// StopEndlessTasks stops the tasks which have running status and continuous updates, but don't end for a long time
func StopEndlessTasks(ctx context.Context) error {
	return stopTasks(ctx, actions_model.FindTaskOptions{
		Status:        []actions_model.Status{actions_model.StatusRunning},
		StartedBefore: timeutil.TimeStamp(time.Now().Add(-setting.Actions.EndlessTaskTimeout).Unix()),
	})
}

func stopTasks(ctx context.Context, opts actions_model.FindTaskOptions) error {
	tasks, err := db.Find[actions_model.ActionTask](ctx, opts)
	if err != nil {
		return fmt.Errorf("find tasks: %w", err)
	}

	jobs := make([]*actions_model.ActionRunJob, 0, len(tasks))
	for _, task := range tasks {
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			if err := StopTask(ctx, task.ID, actions_model.StatusFailure); err != nil {
				return err
			}
			if err := task.LoadJob(ctx); err != nil {
				return err
			}
			jobs = append(jobs, task.Job)
			return nil
		}); err != nil {
			log.Warn("Cannot stop task %v: %v", task.ID, err)
			continue
		}

		remove, err := actions.TransferLogs(ctx, task.LogFilename)
		if err != nil {
			log.Warn("Cannot transfer logs of task %v: %v", task.ID, err)
			continue
		}
		task.LogInStorage = true
		if err := actions_model.UpdateTask(ctx, task, "log_in_storage"); err != nil {
			log.Warn("Cannot update task %v: %v", task.ID, err)
			continue
		}
		remove()
	}

	CreateCommitStatus(ctx, jobs...)

	return nil
}

// CancelAbandonedJobs cancels the jobs which have waiting status, but haven't been picked by a runner for a long time
func CancelAbandonedJobs(ctx context.Context) error {
	jobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{
		Statuses:         []actions_model.Status{actions_model.StatusWaiting, actions_model.StatusBlocked},
		UpdatedBefore:    timeutil.TimeStamp(time.Now().Add(-setting.Actions.AbandonedJobTimeout).Unix()),
		RunNeedsApproval: optional.Some(false),
	})
	if err != nil {
		log.Warn("find abandoned tasks: %v", err)
		return err
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		runsToUpdate := map[int64]*actions_model.ActionRun{}
		now := timeutil.TimeStampNow()
		for _, job := range jobs {
			// Capture the current status because it is required for sending notifications.
			priorStatus := job.Status

			job.Stopped = now
			job.Status = actions_model.StatusCancelled
			if _, err = actions_model.UpdateRunJobWithoutNotification(ctx, job, nil, "status", "stopped"); err != nil {
				return fmt.Errorf("could not cancel abandoned job %d: %v", job.ID, err)
			}

			if err = PropagateJobStatus(ctx, job.ID, priorStatus); err != nil {
				return fmt.Errorf("could not propagate the status of job %d: %w", job.ID, err)
			}

			if err = job.LoadRun(ctx); err != nil {
				return fmt.Errorf("could not load run of job %d: %w", job.ID, err)
			}
			runsToUpdate[job.RunID] = job.Run

			CreateCommitStatus(ctx, job)
		}

		for _, run := range runsToUpdate {
			if err = RefreshAndPropagateRunStatus(ctx, run); err != nil {
				return fmt.Errorf("could not refresh and propagate the status of run %d: %w", run.ID, err)
			}
		}

		return nil
	})
}
