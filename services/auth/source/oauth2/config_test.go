// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_test

import (
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/services/auth/source/oauth2"

	"github.com/stretchr/testify/require"
)

// regression #13478
func TestOAuth2Disabled(t *testing.T) {
	cfg, _ := setting.NewConfigProviderFromData(`
[oauth2]
ENABLED=false
`)
	defer test.MockVariableValue(&setting.CfgProvider, cfg)()
	setting.LoadCommonSettings()
	err := oauth2.Init(t.Context())
	require.NoError(t, err)
}
