// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_test

import (
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/services/auth/source/oauth2"

	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/require"
)

// regressions: Components used even if OAuth2 disabled
func TestOAuth2Disabled(t *testing.T) {
	cfg, _ := setting.NewConfigProviderFromData(`
[oauth2]
ENABLED=false
`)
	defer test.MockVariableValue(&setting.CfgProvider, cfg)()
	setting.LoadCommonSettings()
	err := oauth2.Init(t.Context())
	// #13478
	require.NoError(t, err)

	// #13561 #13563
	require.IsType(t, &oauth2.SessionsStore{}, gothic.Store)
}
