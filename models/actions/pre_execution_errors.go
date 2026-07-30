// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"
	"html"
	"html/template"

	"forgejo.org/modules/translation"
)

type PreExecutionError int64

// PreExecutionError values are stored in the database in ActionRun.PreExecutionError and therefore values can't be
// reordered or changed without a database migration.  Translation arguments are stored in the database in
// PreExecutionErrorDetails, and so they can't be changed or reordered without creating a migration or a new error code
// to represent the new argument details.
const (
	ErrorCodeEventDetectionError PreExecutionError = iota + 1
	ErrorCodeJobParsingError
	ErrorCodePersistentIncompleteMatrix // obsolete
	ErrorCodeIncompleteMatrixMissingJob
	ErrorCodeIncompleteMatrixMissingOutput
	ErrorCodeIncompleteMatrixUnknownCause
	ErrorCodeIncompleteRunsOnMissingJob
	ErrorCodeIncompleteRunsOnMissingOutput
	ErrorCodeIncompleteRunsOnMissingMatrixDimension
	ErrorCodeIncompleteRunsOnUnknownCause
	ErrorCodeIncompleteWithMissingJob
	ErrorCodeIncompleteWithMissingOutput
	ErrorCodeIncompleteWithMissingMatrixDimension
	ErrorCodeIncompleteWithUnknownCause
	ErrorCodeUnknownJobInNeeds
)

func TranslatePreExecutionError(lang translation.Locale, run *ActionRun) template.HTML {
	if run.PreExecutionError != "" {
		// legacy: Forgejo v13 stored value pre-translated, preventing correct translation to active user. HTML escaping
		// the entire string probably isn't right, but we do not have the details to reconstruct the string any smarter
		// than this, so it's a safe default.
		return template.HTML(html.EscapeString(run.PreExecutionError))
	}

	switch run.PreExecutionErrorCode {
	case 0:
		return ""
	case ErrorCodeEventDetectionError:
		return lang.Tr("actions.workflow.event_detection_error", run.PreExecutionErrorDetails...)
	case ErrorCodeJobParsingError:
		return lang.Tr("actions.workflow.job_parsing_error", run.PreExecutionErrorDetails...)
	case ErrorCodePersistentIncompleteMatrix:
		return lang.Tr("actions.workflow.persistent_incomplete_matrix", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteMatrixMissingJob:
		return lang.Tr("actions.workflow.incomplete_matrix_missing_job", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteMatrixMissingOutput:
		return lang.Tr("actions.workflow.incomplete_matrix_missing_output", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteMatrixUnknownCause:
		return lang.Tr("actions.workflow.incomplete_matrix_unknown_cause", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteRunsOnMissingJob:
		return lang.Tr("actions.workflow.incomplete_runson_missing_job", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteRunsOnMissingOutput:
		return lang.Tr("actions.workflow.incomplete_runson_missing_output", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteRunsOnMissingMatrixDimension:
		return lang.Tr("actions.workflow.incomplete_runson_missing_matrix_dimension", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteRunsOnUnknownCause:
		return lang.Tr("actions.workflow.incomplete_runson_unknown_cause", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteWithMissingJob:
		return lang.Tr("actions.workflow.incomplete_with_missing_job", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteWithMissingOutput:
		return lang.Tr("actions.workflow.incomplete_with_missing_output", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteWithMissingMatrixDimension:
		return lang.Tr("actions.workflow.incomplete_with_missing_matrix_dimension", run.PreExecutionErrorDetails...)
	case ErrorCodeIncompleteWithUnknownCause:
		return lang.Tr("actions.workflow.incomplete_with_unknown_cause", run.PreExecutionErrorDetails...)
	case ErrorCodeUnknownJobInNeeds:
		return lang.Tr("actions.workflow.unknown_job_in_needs", run.PreExecutionErrorDetails...)
	}
	return template.HTML(html.EscapeString(fmt.Sprintf("unsupported error: code=%v details=%#v", run.PreExecutionErrorCode, run.PreExecutionErrorDetails)))
}
