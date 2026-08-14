// Copyright 2026 The Forgejo Authors.
// SPDX-License-Identifier: GPLv3-or-later

package integration

import (
	"net/url"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	funding_tests "forgejo.org/services/funding/tests"
)

func TestFundingRetrieval(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.DefaultAllowCreateOrganization, true)()

	onApplicationRun(t, func(t *testing.T, url *url.URL) {
		funding_tests.FromDefaultBranch(t)
	})
}
