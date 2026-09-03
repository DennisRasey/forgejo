// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	"forgejo.org/modules/util"
	notify_service "forgejo.org/services/notify"

	"code.forgejo.org/forgejo/runner/v13/act/jobparser"
)

// InsertRun inserts a new run, and all its jobs, into the database. In the event that all the `if` clauses of the jobs
// are evaluated at this stage and are `false`,
func InsertRun(ctx context.Context, run *actions_model.ActionRun, sw []*jobparser.SingleWorkflow) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		calculateWarnings(run, sw)

		jobs, err := convertSingleWorkflowToJobs(run, sw)
		if err != nil {
			return err
		}

		if err := actions_model.InsertRunWithoutNotification(ctx, run, jobs); err != nil {
			return fmt.Errorf("InsertRunWithoutNotification: %w", err)
		}

		if err = propagateNextRunAttempt(ctx, run.ID); err != nil {
			return fmt.Errorf("failed to propagate next attempt of run %d: %w", run.ID, err)
		}

		for _, job := range jobs {
			if err = PropagateNextJobAttempt(ctx, job.ID); err != nil {
				return fmt.Errorf("failed to propagate new attempt of job %d: %w", job.ID, err)
			}
		}

		// Some jobs might have been immediately set to Skipped when they were inserted.  Other jobs may be
		// dependent on those skipped jobs.  While we're still in this transaction and before these jobs are visible,
		// run the job emitter which can recursively evaluate this state and update dependent runs status to either
		// skipped or waiting, depending on their 'if':
		if !run.NeedApproval { // don't unblock jobs if the run needs approval
			if err := checkJobsOfRun(ctx, run.ID, 0); err != nil {
				return fmt.Errorf("check jobs of run: %w", err)
			}
		}

		// Normally, the status of a job is input to InsertRun as Waiting, and remains that way. But InsertRunJobs can
		// evaluate the 'if' clauses of each job, and if every job is skipped then the run status needs to be updated.
		if err := RefreshAndPropagateRunStatus(ctx, run.ID); err != nil {
			return fmt.Errorf("could not refresh and propagate the status of run %d: %w", run.ID, err)
		}

		// checkJobsOfRun() and RefreshAndPropagateRunStatus() above can lead to an update of the
		// run. But as they load the run from the database, and might even write directly to the
		// database, the changes are not reflected in the `run` variable. Therefore, we have to
		// refresh it.
		dbRun, err := actions_model.GetRunByID(ctx, run.ID)
		if err != nil {
			return fmt.Errorf("could not load run %d: %w", run.ID, err)
		}
		*run = *dbRun

		return nil
	})
}

func propagateNextRunAttempt(ctx context.Context, runID int64) error {
	run, err := actions_model.GetRunByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("could not load run %d: %w", runID, err)
	}

	// Notifications expect a fully loaded run.
	if err := run.LoadAttributes(ctx); err != nil {
		return fmt.Errorf("could not load attributes of run %d: %w", run.ID, err)
	}

	notify_service.NewWorkflowRunAttempt(ctx, run)

	return nil
}

func killRun(ctx context.Context, run *actions_model.ActionRun, newStatus actions_model.Status) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
		if err != nil {
			return fmt.Errorf("could not get jobs of run %d: %w", run.ID, err)
		}
		for _, job := range jobs {
			if err := cancelSingleJob(ctx, job, newStatus); err != nil {
				return err
			}
		}

		if run.NeedApproval {
			if err := actions_model.UpdateRunApprovalByID(ctx, run.ID, actions_model.DoesNotNeedApproval, 0); err != nil {
				return err
			}
		}

		if err = RefreshAndPropagateRunStatus(ctx, run.ID); err != nil {
			return fmt.Errorf("could not refresh and propagate the status of run %d: %w", run.ID, err)
		}

		CreateCommitStatus(ctx, jobs...)

		return nil
	})
}

func CancelRun(ctx context.Context, run *actions_model.ActionRun) error {
	return killRun(ctx, run, actions_model.StatusCancelled)
}

func ApproveRun(ctx context.Context, run *actions_model.ActionRun, doerID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
		if err != nil {
			return err
		}
		for _, job := range jobs {
			if len(job.Needs) == 0 && job.Status.IsBlocked() {
				// Capture the current status because it is required for sending notifications.
				priorStatus := job.Status

				job.Status = actions_model.StatusWaiting
				_, err := actions_model.UpdateRunJobWithoutNotification(ctx, job, nil, "status")
				if err != nil {
					return fmt.Errorf("could not update job %d: %w", job.ID, err)
				}

				if err := PropagateJobStatus(ctx, job.ID, priorStatus); err != nil {
					return fmt.Errorf("could not propagate the status of job %d: %w", job.ID, err)
				}
			}
		}
		CreateCommitStatus(ctx, jobs...)

		if err = RefreshAndPropagateRunStatus(ctx, run.ID); err != nil {
			return fmt.Errorf("could not refresh and propagate the status of run %d: %w", run.ID, err)
		}

		if err = actions_model.UpdateRunApprovalByID(ctx, run.ID, actions_model.DoesNotNeedApproval, doerID); err != nil {
			return fmt.Errorf("failed to update the approval status of run %d: %w", run.ID, err)
		}

		return nil
	})
}

func FailRunPreExecutionError(ctx context.Context, run *actions_model.ActionRun, errorCode actions_model.PreExecutionError, details []any) error {
	if run.PreExecutionErrorCode != 0 {
		// Already have one error; keep it.
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		// The run cannot be marked as failed without marking its job as failed because the run's
		// status is a product of the statuses of its jobs. killRun() will take care of it.
		run.PreExecutionErrorCode = errorCode
		run.PreExecutionErrorDetails = details
		if err := actions_model.UpdateRun(ctx, run, []string{"pre_execution_error_code", "pre_execution_error_details"}...); err != nil {
			return err
		}

		// Mark the run and every pending job as failed so nothing remains in a waiting/blocked state.
		return killRun(ctx, run, actions_model.StatusFailure)
	})
}

// Perform pre-execution checks that would affect the ability for a job to reach an executing stage.
func consistencyCheckRun(ctx context.Context, run *actions_model.ActionRun) error {
	var jobs actions_model.ActionJobList
	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		return err
	}
	validJobIDs := jobs.GetJobIDs()
	for _, job := range jobs {
		if unknownJobIDs, ok := job.AllNeedsExist(validJobIDs); !ok {
			return FailRunPreExecutionError(ctx, run, actions_model.ErrorCodeUnknownJobInNeeds,
				[]any{job.JobID, strings.Join(util.ConvertSlice[actions_model.JobIdentifier, string](unknownJobIDs), ", ")})
		}
		if stop, err := checkJobWillRevisit(ctx, job); err != nil {
			return err
		} else if stop {
			break
		}
		if stop, err := checkJobRunsOnStaticMatrixError(ctx, job); err != nil {
			return err
		} else if stop {
			break
		}
	}
	return nil
}

func checkJobWillRevisit(ctx context.Context, job *actions_model.ActionRunJob) (bool, error) {
	// If a job has a matrix like `${{ needs.other-job.outputs.some-output }}`, it will be marked as an
	// `IncompleteMatrix` job until the `other-job` is completed, and it will be marked as StatusBlocked; then when
	// `other-job` is completed, the job_emitter will check dependent jobs and revisit them.  But, it's possible that
	// the job didn't list `other-job` in its `needs: [...]` list -- in this case, a job will be marked as StatusBlocked
	// forever.
	//
	// Check to ensure that a job marked with `IncompleteMatrix` doesn't refer to a job that it doesn't have listed in
	// `needs`.  If that state is discovered, fail the job and mark a PreExecutionError on the run.

	isIncompleteMatrix, matrixNeeds, err := job.HasIncompleteMatrix()
	if err != nil {
		return false, err
	}

	if !isIncompleteMatrix || matrixNeeds == nil {
		// Not actually IncompleteMatrix, or has no information about the `${{ needs... }}` reference, nothing we can do
		// here.
		return false, nil
	}

	requiredJob := actions_model.LocalJobIdentifier(matrixNeeds.Job)
	needs := job.Needs
	if slices.Contains(needs, requiredJob) {
		// Looks good, the needed job is listed in `needs`.  It's possible that the matrix may be incomplete by
		// referencing multiple different outputs, and not *all* outputs are in the job's `needs`... `requiredJob` will
		// only be the first one that was found while evaluating the matrix.  But as long as at least one job is listed
		// in `needs`, the job should be revisited by job_emitter and end up at a final resolution.
		return false, nil
	}

	// Job doesn't seem like it can proceed; mark the run with an error.
	if err := job.LoadRun(ctx); err != nil {
		return false, err
	}
	if err := FailRunPreExecutionError(ctx, job.Run, actions_model.ErrorCodeIncompleteMatrixMissingJob, []any{
		job.JobID,
		requiredJob,
		strings.Join(util.ConvertSlice[actions_model.LocalJobIdentifier, string](needs), ", "),
	}); err != nil {
		return false, err
	}

	return true, nil
}

func checkJobRunsOnStaticMatrixError(ctx context.Context, job *actions_model.ActionRunJob) (bool, error) {
	// If a job has a `runs-on` field that references a matrix dimension like `runs-on: ${{ matrix.platform }}`, and
	// `platform` is not part of the job's matrix at all, then it will be tagged as `HasIncompleteRunsOn` and will be
	// blocked forever.  This only applies if the matrix is static -- that is, the job isn't also tagged
	// `HasIncompleteMatrix` and the matrix is yet to be fully defined.

	isIncompleteRunsOn, _, matrixReference, err := job.HasIncompleteRunsOn()
	if err != nil {
		return false, err
	} else if !isIncompleteRunsOn || matrixReference == nil {
		// Not incomplete, or, it's incomplete but not because of a matrix reference error.
		return false, nil
	}

	isIncompleteMatrix, _, err := job.HasIncompleteMatrix()
	if err != nil {
		return false, err
	} else if isIncompleteMatrix {
		// Not a static matrix, so this might be resolved later when the job is expanded.
		return false, nil
	}

	// Job doesn't seem like it can proceed; mark the run with an error.
	if err := job.LoadRun(ctx); err != nil {
		return false, err
	}
	if err := FailRunPreExecutionError(ctx, job.Run, actions_model.ErrorCodeIncompleteRunsOnMissingMatrixDimension, []any{
		job.JobID,
		matrixReference.Dimension,
	}); err != nil {
		return false, err
	}

	return true, nil
}

// DeleteRun removes a particular run including all associated artifacts, jobs, tasks, and logs. The run has to be
// completed for the operation to succeed.
func DeleteRun(ctx context.Context, runID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		run, err := actions_model.GetRunByID(ctx, runID)
		if err != nil {
			return fmt.Errorf("unable to load run %d: %w", runID, err)
		}

		if !run.Status.IsDone() {
			return fmt.Errorf("cannot delete run %d because it has not completed yet", run.ID)
		}

		err = actions_model.SetArtifactsOfRunDeleted(ctx, runID)
		if err != nil {
			return fmt.Errorf("unable to delete artifacts of run %d: %w", run.ID, err)
		}

		err = deleteJobsOfRun(ctx, run.ID)
		if err != nil {
			return fmt.Errorf("unable to delete jobs of run %d: %w", run.ID, err)
		}

		return actions_model.DeleteRun(ctx, run.ID)
	})
}

// PrioritizeRun marks the workflow run identified by the given ID as prioritized and recalculates the priority of each
// run in the queue.
func PrioritizeRun(ctx context.Context, run *actions_model.ActionRun) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if run == nil {
			return errors.New("run is nil")
		}

		if run.Prioritize {
			// Run is already prioritized. There is nothing left to do.
			return nil
		}

		run.Prioritize = true
		if err := actions_model.UpdateRun(ctx, run, []string{"prioritize"}...); err != nil {
			return fmt.Errorf("could not update workflow run %d to prioritize: %w", run.ID, err)
		}

		if err := recalculateRunPriorities(ctx, run.RepoID); err != nil {
			return fmt.Errorf("could not recalculate workflow run priorities of repository %d: %w", run.RepoID, err)
		}
		return nil
	})
}

// DeprioritizeRun removes the prioritized mark from a workflow run, if present, and recalculates the priority of each
// run in the queue.
func DeprioritizeRun(ctx context.Context, run *actions_model.ActionRun) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if run == nil {
			return errors.New("run is nil")
		}

		if !run.Prioritize {
			// Run is already not prioritized. There is nothing left to do.
			return nil
		}

		run.Prioritize = false
		if err := actions_model.UpdateRun(ctx, run, []string{"prioritize"}...); err != nil {
			return fmt.Errorf("could not update workflow run %d to deprioritize: %w", run.ID, err)
		}

		if err := recalculateRunPriorities(ctx, run.RepoID); err != nil {
			return fmt.Errorf("could not recalculate workflow run priorities of repository %d: %w", run.RepoID, err)
		}

		return nil
	})
}

// recalculateRunPriorities recalculates the priority of all queued workflow runs that belong to the given repository.
var recalculateRunPriorities = func(ctx context.Context, repoID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		runs, err := actions_model.GetQueuedRunsByRepoID(ctx, repoID)
		if err != nil {
			return fmt.Errorf("could not read pending workflow runs of repository %d: %w", repoID, err)
		}

		strategy := actions_model.DefaultPrioritizationStrategy{}
		updatedRuns, err := strategy.PrioritizeRuns(runs)
		if err != nil {
			return fmt.Errorf("failed to prioritize pending workflow runs of repository %d: %w", repoID, err)
		}

		for _, run := range runs {
			if !updatedRuns.Contains(run.ID) {
				continue
			}

			if err = actions_model.UpdateRun(ctx, run, []string{"priority"}...); err != nil {
				return fmt.Errorf("failed to update reprioritized workflow run %d: %w", run.ID, err)
			}
		}

		// In the future notify webhook listeners. Pass *all* runs, not only updated runs to provide listeners a
		// complete view.

		return nil
	})
}

func InitiateNextRunAttempt(ctx context.Context, run *actions_model.ActionRun) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := run.PrepareNextAttempt(); err != nil {
			return fmt.Errorf("could not prepare next attempt of run %d: %w", run.ID, err)
		}

		if err := actions_model.UpdateRun(ctx, run); err != nil {
			return fmt.Errorf("unable to update run %d: %w", run.ID, err)
		}

		if err := propagateNextRunAttempt(ctx, run.ID); err != nil {
			return fmt.Errorf("failed to propagate next attempt of run %d: %w", run.ID, err)
		}

		return nil
	})
}

// RefreshAndPropagateRunStatus refreshes the status of a run and notifies subscribers if the
// status has changed — but only then.
func RefreshAndPropagateRunStatus(ctx context.Context, runID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		run, err := actions_model.GetRunByID(ctx, runID)
		if err != nil {
			return fmt.Errorf("could not load run %d: %w", runID, err)
		}

		jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
		if err != nil {
			return fmt.Errorf("could not get jobs of run %d: %w", run.ID, err)
		}

		// If the status has not changed, updating the run or triggering notifications is
		// unnecessary.
		priorStatus := run.Status
		if !run.RefreshStatus(jobs) {
			return nil
		}

		if err = actions_model.UpdateRun(ctx, run); err != nil {
			return fmt.Errorf("could not update run %d: %w", run.ID, err)
		}

		// Notifications expect an ActionRun with all its attributes loaded.
		if err = run.LoadAttributes(ctx); err != nil {
			return fmt.Errorf("failed to load attributes of run %d: %w", run.ID, err)
		}

		if !run.Status.IsDone() {
			notify_service.WorkflowRunStatusChanged(ctx, run, priorStatus)
		} else {
			notify_service.WorkflowRunCompleted(ctx, run, priorStatus)
		}

		return nil
	})
}
