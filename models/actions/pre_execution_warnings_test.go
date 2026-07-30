// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"html/template"
	"testing"

	"forgejo.org/modules/translation"

	"github.com/stretchr/testify/assert"
)

func TestTranslatePreExecutionWarning(t *testing.T) {
	translation.InitLocales(t.Context())
	lang := translation.NewLocale("en-US")

	tests := []struct {
		name     string
		run      *ActionRun
		expected []template.HTML
	}{
		{
			name:     "no warning",
			run:      &ActionRun{},
			expected: nil,
		},
		{
			name: "unexpected warning",
			run: &ActionRun{
				PreExecutionWarningCodes: []PreExecutionWarning{-10000},
				PreExecutionWarningDetails: [][]any{
					{"<img src=x onerror=alert(document.domain)>"},
				},
			},
			expected: []template.HTML{"unsupported warning: code=-10000 details=[]interface {}{&#34;&lt;img src=x onerror=alert(document.domain)&gt;&#34;}"},
		},
		{
			name: "WarningCodePermissions",
			run: &ActionRun{
				PreExecutionWarningCodes: []PreExecutionWarning{WarningCodePermissions},
				PreExecutionWarningDetails: [][]any{
					{"job1", "https://forgejo.org/docs/latest/user/authorized-integrations/"},
				},
			},
			expected: []template.HTML{"Job <code>job1</code> or its workflow has a <code>permissions</code> field, which is not supported in Forgejo and will be ignored. Use <a href=\"https://forgejo.org/docs/latest/user/authorized-integrations/\">Authorized Integrations</a> to grant capabilities to this job instead."},
		},
		{
			name: "WarningCodePermissionsEncoding",
			run: &ActionRun{
				PreExecutionWarningCodes: []PreExecutionWarning{WarningCodePermissions},
				PreExecutionWarningDetails: [][]any{
					// job IDs are arbitrary YAML strings, even though we don't often treat them this way:
					{"<img src=x onerror=alert(document.domain)>", "https://forgejo.org/docs/latest/user/authorized-integrations/"},
				},
			},
			expected: []template.HTML{"Job <code>&lt;img src=x onerror=alert(document.domain)&gt;</code> or its workflow has a <code>permissions</code> field, which is not supported in Forgejo and will be ignored. Use <a href=\"https://forgejo.org/docs/latest/user/authorized-integrations/\">Authorized Integrations</a> to grant capabilities to this job instead."},
		},
		{
			name: "MultipleWarnings",
			run: &ActionRun{
				PreExecutionWarningCodes: []PreExecutionWarning{WarningCodePermissions, WarningCodePermissions},
				PreExecutionWarningDetails: [][]any{
					{"job1", "https://forgejo.org/docs/latest/user/authorized-integrations/"},
					{"job4", "https://forgejo.org/docs/latest/user/authorized-integrations/"},
				},
			},
			expected: []template.HTML{
				"Job <code>job1</code> or its workflow has a <code>permissions</code> field, which is not supported in Forgejo and will be ignored. Use <a href=\"https://forgejo.org/docs/latest/user/authorized-integrations/\">Authorized Integrations</a> to grant capabilities to this job instead.",
				"Job <code>job4</code> or its workflow has a <code>permissions</code> field, which is not supported in Forgejo and will be ignored. Use <a href=\"https://forgejo.org/docs/latest/user/authorized-integrations/\">Authorized Integrations</a> to grant capabilities to this job instead.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TranslatePreExecutionWarning(lang, tt.run)
			assert.Equal(t, tt.expected, err)
		})
	}
}
