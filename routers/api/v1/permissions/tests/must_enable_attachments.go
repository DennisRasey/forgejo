// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package tests

import (
	"testing"

	"forgejo.org/modules/setting"
	apiv1_permissions "forgejo.org/routers/api/v1/permissions"
)

var _ = registerFunctionTest(apiv1_permissions.MustEnableAttachments, functionTest{
	testCases: []*testCase{
		{
			// pass if attachments are enabled in settings
			data: newTestData(map[string]string{}, newSharedData()),
		},
		{
			// fail if attachments are disabled in settings
			data: newTestData(map[string]string{
				"Attachment.Enabled": "false",
			}, newSharedData()),
			error: "Not Found",
		},
	},
	protectSettingsBool: []*bool{
		&setting.Attachment.Enabled,
	},
	interpret: func(t *testing.T, permissions *apiv1_permissions.Permissions, data *testData) {
		setting.Attachment.Enabled = data.Get("Attachment.Enabled") != "false"
	},
})
