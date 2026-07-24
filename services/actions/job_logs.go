// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	actions_model "forgejo.org/models/actions"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/actions"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/util"
)

// Sentinel errors returned by OpenJobLogReader. The HTTP handler maps each
// of these (and util.ErrNotExist) to 404; anything else is a 500.
var (
	ErrJobNotExecuted = errors.New("job has not been executed yet")
	ErrLogsExpired    = errors.New("logs have expired")
	ErrStepOutOfRange = errors.New("step index out of range")
)

// OpenJobLogReader returns a reader for an action job's log along with the
// filename and modtime to expose via Content-Disposition / Last-Modified.
//
// attempt, when set, selects a specific historical attempt (uses
// GetTaskByJobAttempt). When unset, the latest attempt is used via the
// job.TaskID pointer maintained by the runner.
//
// stepFilter, when set, narrows the returned bytes to the slice covered by
// that step in the FullSteps numbering: 0 is the "Set up job" head, the
// last index is the "Complete job" tail, real steps are in between. Range
// requests still work via http.ServeContent over the bounded window.
//
// The caller is responsible for closing the returned reader.
func OpenJobLogReader(
	ctx context.Context,
	repo *repo_model.Repository,
	jobID int64,
	attempt optional.Option[int64],
	stepFilter optional.Option[int],
) (io.ReadSeekCloser, string, time.Time, error) {
	job, err := actions_model.GetRunJobByID(ctx, jobID)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	// Run-jobs live in their own table; enforce repo ownership here so the
	// API layer can stay thin.
	if job.RepoID != repo.ID {
		return nil, "", time.Time{}, util.ErrNotExist
	}

	hasAttempt, attemptVal := attempt.Get()
	hasStep, stepIdx := stepFilter.Get()

	var task *actions_model.ActionTask
	switch {
	case hasAttempt:
		task, err = actions_model.GetTaskByJobAttempt(ctx, job.ID, attemptVal)
		if err != nil {
			return nil, "", time.Time{}, err
		}
	case job.TaskID == 0:
		// Job exists, but no runner has picked it up yet (or a re-run has
		// zeroed TaskID and the next runner hasn't claimed it).
		return nil, "", time.Time{}, ErrJobNotExecuted
	default:
		task, err = actions_model.GetTaskByID(ctx, job.TaskID)
		if err != nil {
			return nil, "", time.Time{}, err
		}
	}

	if task.LogExpired {
		return nil, "", time.Time{}, ErrLogsExpired
	}

	modtime := task.Stopped.AsTime()
	if task.Stopped == 0 {
		modtime = task.Updated.AsTime() // Best-guess modtime while still running.
	}

	if !hasStep {
		reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
		if err != nil {
			return nil, "", time.Time{}, fmt.Errorf("open logs for task %d: %w", task.ID, err)
		}
		filename := fmt.Sprintf("job-%d-attempt-%d.log", job.ID, task.Attempt)
		return reader, filename, modtime, nil
	}

	// Resolve the step's byte window BEFORE opening the log reader so a
	// request that's going to 404 doesn't pay for an OpenLogs call. Targeted
	// step load — task.LoadAttributes would also pull job + run which we
	// don't need (and which makes unit-test setup heavier).
	steps, err := actions_model.GetTaskStepsByTaskID(ctx, task.ID)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("could not load steps for task %d: %w", task.ID, err)
	}
	task.Steps = steps
	full := actions.FullSteps(task)
	if stepIdx < 0 || stepIdx >= len(full) {
		return nil, "", time.Time{}, ErrStepOutOfRange
	}
	startByte, endByte := actions.StepByteRange(task, full[stepIdx])
	reader, err := actions.OpenLogsRange(ctx, task.LogInStorage, task.LogFilename, startByte, endByte-startByte)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("open logs for task %d step %d: %w", task.ID, stepIdx, err)
	}
	stepFilename := fmt.Sprintf("job-%d-attempt-%d-step-%d.log", job.ID, task.Attempt, stepIdx)
	return reader, stepFilename, modtime, nil
}

// WriteRunLogsZip writes a ZIP of the latest per-job logs for the run to w.
// Each entry is named {job-name}-{job-id}-attempt-{N}.log, where N is that
// job's current attempt — the run itself has no attempt number, so jobs that
// were re-run independently show different attempts here. Jobs that haven't
// run, can't be looked up, or have expired logs get a .MISSING marker; a
// mid-stream read failure gets a sibling .PARTIAL marker. Any ZIP-level
// write failure (e.g. the HTTP client disconnects mid-stream) is propagated
// so the caller can abort instead of churning through the remaining jobs.
// Caller sets Content-Type / Content-Disposition before calling.
func WriteRunLogsZip(ctx context.Context, w io.Writer, run *actions_model.ActionRun) error {
	jobs, err := actions_model.GetRunJobsByRunID(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("get jobs for run %d: %w", run.ID, err)
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	// strip control bytes and path separators; UTF-8 passes through.
	sanitize := func(name string) string {
		cleaned := strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
				return -1
			}
			return r
		}, name)
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			cleaned = "job"
		}
		return cleaned
	}

	entryName := func(job *actions_model.ActionRunJob, suffix string) string {
		return fmt.Sprintf("%s-%d-attempt-%d.%s", sanitize(job.Name), job.ID, job.Attempt, suffix)
	}

	writeMarker := func(job *actions_model.ActionRunJob, suffix, msg string) error {
		entry, werr := zw.Create(entryName(job, suffix))
		if werr != nil {
			return werr
		}
		_, werr = entry.Write([]byte(msg))
		return werr
	}

	// Inner closure so reader.Close runs per iteration via defer.
	processJob := func(job *actions_model.ActionRunJob) error {
		if job.TaskID == 0 {
			return writeMarker(job, "MISSING", "job has not been executed yet\n")
		}
		task, err := actions_model.GetTaskByID(ctx, job.TaskID)
		if err != nil {
			return writeMarker(job, "MISSING", fmt.Sprintf("task lookup failed: %v\n", err))
		}
		if task.LogExpired {
			return writeMarker(job, "MISSING", "logs have been cleaned up\n")
		}

		reader, err := actions.OpenLogs(ctx, task.LogInStorage, task.LogFilename)
		if err != nil {
			return writeMarker(job, "MISSING", fmt.Sprintf("log open failed: %v\n", err))
		}
		defer reader.Close()

		entry, err := zw.Create(entryName(job, "log"))
		if err != nil {
			return writeMarker(job, "MISSING", fmt.Sprintf("zip entry create failed: %v\n", err))
		}

		if _, copyErr := io.Copy(entry, reader); copyErr != nil {
			return writeMarker(job, "PARTIAL", fmt.Sprintf("log read failed mid-stream: %v\n", copyErr))
		}
		return nil
	}

	for _, job := range jobs {
		if err := processJob(job); err != nil {
			return err
		}
	}
	return nil
}
