// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package setting_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/testlogger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFundingProviderConfigValidDefaultTemplates(t *testing.T) {
	// If FUNDING_TEST_LIVE_PROVIDERS is set, this test will make HTTP requests to the live funding provider URLs.
	// When doing so, the responses will be forgotten (saved to a garbage directory).
	// If the var is not set, this test does nothing.
	fundingTestLiveProviders := os.Getenv("FUNDING_TEST_LIVE_PROVIDERS")
	testDataDir := t.TempDir()

	if fundingTestLiveProviders == "" {
		return
	}

	// Accounts on our built-in providers which we expect to exist:
	cases := [][2]string{
		{"community_bridge", "kubernetes"},
		{"github", "feross"},                // from our package-lock.json
		{"issuehunt", "bk138"},              // from https://github.com/bk138/droidVNC-NG
		{"ko_fi", "browniebroke"},           // from https://github.com/browniebroke/deezer-python
		{"liberapay", "forgejo"},            // 👋
		{"open_collective", "eslint"},       // from our package-lock.json
		{"patreon", "feross"},               // from our package-lock.json
		{"tidelift", "npm/browserslist"},    // from our package-lock.json
		{"buy_me_a_coffee", "browniebroke"}, // from https://github.com/browniebroke/deezer-python
		// "thanks_dev" pages always return HTTP 403, so we test it instead at tests/e2e/funding-config.test.e2e.ts
		// "custom" is untestable here; funding config authors are expected to test their own links
	}

	setting.LoadBuiltInFundingProviders()
	for _, c := range cases {
		name := c[0]
		input := c[1]

		provider := setting.FundingProviders[name]
		require.NotNil(t, provider)
		url := fmt.Sprintf(provider.Template, input)

		t.Run(name, func(t *testing.T) {
			defer testlogger.PrintCurrentTest(t)()

			server := unittest.NewMockWebServer(t, url, testDataDir, true)
			defer server.Close()

			resp, err := server.Client().Get(server.URL)
			require.NoError(t, err)

			// We expect either HTTP 200 exactly, or some kind of redirect (3XX)
			statusCode := resp.StatusCode
			assert.True(t, statusCode == http.StatusOK || (statusCode > 300 && statusCode < 400), "unexpected status code for %s: %d is not 200 or between 300 and 399 inclusive", url, statusCode)
		})
	}
}
