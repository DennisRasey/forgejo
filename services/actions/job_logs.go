// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	actions_model "forgejo.org/models/actions"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/actions"
	"forgejo.org/modules/json"
	"forgejo.org/modules/log"
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

// JobLogFilterOptions configures how WriteJobLogStream transforms its input.
type JobLogFilterOptions struct {
	// Query, if non-empty, filters output to lines whose content (the part
	// after the timestamp prefix) contains the substring. Regex is not
	// supported; the match is plain strings.Contains.
	Query string
	// IgnoreCase makes Query a case-insensitive substring match.
	IgnoreCase bool
	// JSON switches the output format from plaintext lines to NDJSON, where
	// each emitted line is a {time, content} object on its own line. When
	// false, matched lines are written verbatim in the storage form
	// (actions.FormatLog output) — the cheap text path stays on
	// http.ServeContent and never calls this.
	JSON bool
}

// jsonLine is the NDJSON shape emitted by WriteJobLogStream when opts.JSON
// is true. time.Time marshals as RFC3339Nano via the configured json module.
type jsonLine struct {
	Time    time.Time `json:"time"`
	Content string    `json:"content"`
}

// WriteJobLogStream scans reader line-by-line and writes a filtered/optionally
// re-encoded view to w. The reader is assumed to already cover exactly the
// byte range the caller wants to scan (the underlying log, or a step-bounded
// window). When opts.Query is empty AND opts.JSON is false the function is
// effectively io.Copy with line buffering; the cheap text path stays on
// http.ServeContent and never calls this.
func WriteJobLogStream(w io.Writer, reader io.Reader, opts JobLogFilterOptions) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, actions.MaxStoredLineSize), actions.MaxStoredLineSize)

	needle := opts.Query
	if opts.IgnoreCase {
		needle = strings.ToLower(needle)
	}

	var enc json.Encoder
	if opts.JSON {
		enc = json.NewEncoder(w)
	}

	for scanner.Scan() {
		raw := scanner.Bytes()
		var ts time.Time
		var content string
		needParse := needle != "" || opts.JSON
		if needParse {
			t, c, err := actions.ParseLog(scanner.Text())
			if err != nil {
				// Malformed line; safe to skip. Storage writes well-formed
				// lines, so this should be rare and tolerable -- but log so
				// operators have something to attach when users report missing
				// lines.
				log.Warn("WriteJobLogStream: skipping malformed line: %v", err)
				continue
			}
			ts, content = t, c
		}
		if needle != "" {
			hay := content
			if opts.IgnoreCase {
				hay = strings.ToLower(hay)
			}
			if !strings.Contains(hay, needle) {
				continue
			}
		}
		if opts.JSON {
			if err := enc.Encode(jsonLine{Time: ts.UTC(), Content: content}); err != nil {
				return fmt.Errorf("could not encode NDJSON line: %w", err)
			}
			continue
		}
		if _, err := w.Write(raw); err != nil {
			return fmt.Errorf("could not write log line: %w", err)
		}
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return fmt.Errorf("could not write log newline: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("could not scan log stream: %w", err)
	}
	return nil
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
