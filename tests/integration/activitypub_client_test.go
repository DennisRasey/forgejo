// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"testing"
	"time"

	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/activitypub"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/routers"
	"forgejo.org/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityPubClientBodySize(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		url := u.JoinPath("/api/v1/nodeinfo").String()

		ctx, _ := contexttest.MockAPIContext(t, url)
		clientFactory, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		apClient, err := clientFactory.WithKeys(ctx, user1, user1.KeyID(), nil)
		require.NoError(t, err)

		// Request with normal MaxSize
		t.Run("NormalMaxSize", func(t *testing.T) {
			resp, err := apClient.GetBody(url)
			require.NoError(t, err)
			assert.Contains(t, string(resp), "forgejo")
		})

		// Set MaxSize to something very low to always fail
		// Request with low MaxSize
		t.Run("LowMaxSize", func(t *testing.T) {
			defer test.MockVariableValue(&setting.Federation.MaxSize, 100)()

			_, err = apClient.GetBody(url)
			require.Error(t, err)
			assert.ErrorContains(t, err, "Request returned")
		})
	})
}
