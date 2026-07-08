// Copyright 2024, 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0

package integration

import (
	"fmt"
	"net/url"
	"strconv"
	"testing"
	"time"

	"forgejo.org/modules/activitypub"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/routers"
	"forgejo.org/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityPubHostMatcher(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.SignatureEnforced, true)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, false)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	mock := test.NewFederationServerMock()
	federatedSrv := mock.DistantServer(t)
	defer federatedSrv.Close()

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		ctx, _ := contexttest.MockAPIContext(t, u.String())
		cf, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		repositoryID := 2
		userID := 2

		repositoryPath := fmt.Sprintf("repository-id/%d", repositoryID)
		userPath := fmt.Sprintf("user-id/%d", userID)

		for _, actorPath := range []string{repositoryPath, userPath} {
			for _, internalHost := range []string{"http://127.0.0.1", "http://localhost"} {
				// confirm that even setting the allow-list to an internal host still blocks local hosts
				internalURI, err := url.Parse(internalHost)
				require.NoError(t, err)

				c, err := cf.WithKeysDirect(ctx, mock.Persons[0].PrivKey,
					mock.Persons[0].KeyID(federatedSrv.URL), []*url.URL{internalURI})
				require.NoError(t, err)

				// avoid calls to internal IPs with invalid ports
				resp, err := c.GetBody(fmt.Sprintf("https://127.0.0.1:9999/api/v1/activitypub/%s", actorPath))
				require.Error(t, err)
				assert.Nil(t, resp)

				resp, err = c.GetBody(fmt.Sprintf("http://127.0.0.1:5432/api/v1/activitypub/%s", actorPath))
				require.Error(t, err)
				assert.Nil(t, resp)

				// avoid calls to internal loopback with invalid ports
				resp, err = c.GetBody(fmt.Sprintf("http://localhost/api/v1/activitypub/%s", actorPath))
				require.Error(t, err)
				assert.Nil(t, resp)

				resp, err = c.GetBody(fmt.Sprintf("http://localhost:5432/api/v1/activitypub/%s", actorPath))
				require.Error(t, err)
				assert.Nil(t, resp)

				resp, err = c.GetBody(fmt.Sprintf("http://loopback/api/v1/activitypub/%s", actorPath))
				require.Error(t, err)
				assert.Nil(t, resp)

				resp, err = c.GetBody(fmt.Sprintf("http://loopback:5432/api/v1/activitypub/%s", actorPath))
				require.Error(t, err)
				assert.Nil(t, resp)

				// avoid port-knocking on unexpected ports
				resp, err = c.GetBody(fmt.Sprintf("%s:5432/api/v1/activitypub/%s", u.Hostname(), actorPath))
				require.Error(t, err)
				assert.Nil(t, resp)

				if port, err := strconv.Atoi(u.Port()); err == nil {
					resp, err = c.GetBody(fmt.Sprintf("%s:%d/api/v1/activitypub/%s", u.Hostname(), port+1, actorPath))
					require.Error(t, err)
					assert.Nil(t, resp)

					resp, err = c.GetBody(fmt.Sprintf("%s:%d/api/v1/activitypub/%s", u.Hostname(), port-1, actorPath))
					require.Error(t, err)
					assert.Nil(t, resp)
				}
			}
		}
	})
}
