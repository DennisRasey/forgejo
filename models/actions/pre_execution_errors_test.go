// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"html/template"
	"testing"

	"forgejo.org/modules/translation"

	"github.com/stretchr/testify/assert"
)

func TestTranslatePreExecutionError(t *testing.T) {
	translation.InitLocales(t.Context())
	lang := translation.NewLocale("en-US")

	tests := []struct {
		name     string
		run      *ActionRun
		expected template.HTML
	}{
		{
			name:     "legacy",
			run:      &ActionRun{PreExecutionError: "legacy message"},
			expected: "legacy message",
		},
		{
			name:     "no error",
			run:      &ActionRun{},
			expected: "",
		},
		{
			name: "unexpected error",
			run: &ActionRun{
				PreExecutionErrorCode:    PreExecutionError(-10000),
				PreExecutionErrorDetails: []any{"<img src=x onerror=alert(document.domain)>"},
			},
			expected: "unsupported error: code=-10000 details=[]interface {}{&#34;&lt;img src=x onerror=alert(document.domain)&gt;&#34;}",
		},
		{
			name: "ErrorCodeEventDetectionError",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeEventDetectionError,
				PreExecutionErrorDetails: []any{"inner error message"},
			},
			expected: "Unable to parse supported events in workflow: inner error message",
		},
		{
			name: "ErrorCodeJobParsingError",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeJobParsingError,
				PreExecutionErrorDetails: []any{"inner error message"},
			},
			expected: "Unable to parse jobs in workflow: inner error message",
		},
		{
			name: "ErrorCodePersistentIncompleteMatrix",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodePersistentIncompleteMatrix,
				PreExecutionErrorDetails: []any{"blocked_job", "needs-1, needs-2"},
			},
			expected: "Unable to evaluate `strategy.matrix` of job blocked_job due to a `needs` expression that was invalid. It may reference a job that is not in it's 'needs' list (needs-1, needs-2), or an output that doesn't exist on one of those jobs.",
		},
		{
			name: "ErrorCodePersistentIncompleteMatrixEncoding",
			run: &ActionRun{
				PreExecutionErrorCode: ErrorCodePersistentIncompleteMatrix,
				// job IDs are arbitrary YAML strings, even though we don't often treat them this way:
				PreExecutionErrorDetails: []any{"<img src=x onerror=alert(document.domain)>", "needs-1, needs-2"},
			},
			expected: "Unable to evaluate `strategy.matrix` of job &lt;img src=x onerror=alert(document.domain)&gt; due to a `needs` expression that was invalid. It may reference a job that is not in it's 'needs' list (needs-1, needs-2), or an output that doesn't exist on one of those jobs.",
		},
		{
			name: "ErrorCodeIncompleteMatrixMissingOutput",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteMatrixMissingOutput,
				PreExecutionErrorDetails: []any{"blocked_job", "other_job", "some_output"},
			},
			expected: "Unable to evaluate `strategy.matrix` of job blocked_job: job other_job is missing output some_output.",
		},
		{
			name: "ErrorCodeIncompleteMatrixMissingJob",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteMatrixMissingJob,
				PreExecutionErrorDetails: []any{"blocked_job", "other_job", "needs-1, needs-2"},
			},
			expected: "Unable to evaluate `strategy.matrix` of job blocked_job: job other_job is not in the `needs` list of job blocked_job (needs-1, needs-2).",
		},
		{
			name: "ErrorCodeIncompleteMatrixUnknownCause",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteMatrixUnknownCause,
				PreExecutionErrorDetails: []any{"blocked_job"},
			},
			expected: "Unable to evaluate `strategy.matrix` of job blocked_job: unknown error.",
		},
		{
			name: "ErrorCodeIncompleteRunsOnMissingOutput",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteRunsOnMissingOutput,
				PreExecutionErrorDetails: []any{"blocked_job", "other_job", "some_output"},
			},
			expected: "Unable to evaluate `runs-on` of job blocked_job: job other_job is missing output some_output.",
		},
		{
			name: "ErrorCodeIncompleteRunsOnMissingJob",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteRunsOnMissingJob,
				PreExecutionErrorDetails: []any{"blocked_job", "other_job", "needs-1, needs-2"},
			},
			expected: "Unable to evaluate `runs-on` of job blocked_job: job other_job is not in the `needs` list of job blocked_job (needs-1, needs-2).",
		},
		{
			name: "ErrorCodeIncompleteRunsOnMissingMatrixDimension",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteRunsOnMissingMatrixDimension,
				PreExecutionErrorDetails: []any{"blocked_job", "platfurm"},
			},
			expected: "Unable to evaluate `runs-on` of job blocked_job: matrix dimension platfurm does not exist.",
		},
		{
			name: "ErrorCodeIncompleteRunsOnUnknownCause",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteRunsOnUnknownCause,
				PreExecutionErrorDetails: []any{"blocked_job"},
			},
			expected: "Unable to evaluate `runs-on` of job blocked_job: unknown error.",
		},
		{
			name: "ErrorCodeIncompleteWithMissingOutput",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteWithMissingOutput,
				PreExecutionErrorDetails: []any{"blocked_job", "other_job", "some_output"},
			},
			expected: "Unable to evaluate `with` of job blocked_job: job other_job is missing output some_output.",
		},
		{
			name: "ErrorCodeIncompleteWithMissingJob",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteWithMissingJob,
				PreExecutionErrorDetails: []any{"blocked_job", "other_job", "needs-1, needs-2"},
			},
			expected: "Unable to evaluate `with` of job blocked_job: job other_job is not in the `needs` list of job blocked_job (needs-1, needs-2).",
		},
		{
			name: "ErrorCodeIncompleteWithMissingMatrixDimension",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteWithMissingMatrixDimension,
				PreExecutionErrorDetails: []any{"blocked_job", "platfurm"},
			},
			expected: "Unable to evaluate `with` of job blocked_job: matrix dimension platfurm does not exist.",
		},
		{
			name: "ErrorCodeIncompleteWithUnknownCause",
			run: &ActionRun{
				PreExecutionErrorCode:    ErrorCodeIncompleteWithUnknownCause,
				PreExecutionErrorDetails: []any{"blocked_job"},
			},
			expected: "Unable to evaluate `with` of job blocked_job: unknown error.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TranslatePreExecutionError(lang, tt.run)
			assert.Equal(t, tt.expected, err)
		})
	}
}
