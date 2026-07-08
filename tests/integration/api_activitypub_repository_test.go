// Copyright 2024, 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"forgejo.org/models/forgefed"
	"forgejo.org/models/unittest"
	"forgejo.org/models/user"
	"forgejo.org/modules/activitypub"
	forgefed_modules "forgejo.org/modules/forgefed"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/routers"
	"forgejo.org/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityPubRepository(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	mock := test.NewFederationServerMock()
	federatedSrv := mock.DistantServer(t)
	defer federatedSrv.Close()

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		repositoryID := 2

		localRepository := fmt.Sprintf("%sapi/v1/activitypub/repository-id/%d", u, repositoryID)

		ctx, _ := contexttest.MockAPIContext(t, localRepository)
		cf, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		c, err := cf.WithKeysDirect(ctx, mock.Persons[0].PrivKey,
			mock.Persons[0].KeyID(federatedSrv.URL), nil)
		require.NoError(t, err)

		resp, err := c.GetBody(localRepository)
		require.NoError(t, err)
		assert.Contains(t, string(resp), "@context")

		var repository forgefed_modules.Repository
		err = repository.UnmarshalJSON(resp)
		require.NoError(t, err)

		assert.Regexp(t, fmt.Sprintf("activitypub/repository-id/%d$", repositoryID), repository.GetID().String())
	})
}

func TestActivityPubMissingRepository(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.SignatureEnforced, false)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	repositoryID := 9999999
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/activitypub/repository-id/%d", repositoryID))
	resp := MakeRequest(t, req, http.StatusNotFound)
	assert.Contains(t, resp.Body.String(), "repository does not exist")
}

func TestActivityPubRepositoryInboxValid(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.SignatureEnforced, true)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	mock := test.NewFederationServerMock()
	federatedSrv := mock.DistantServer(t)
	defer federatedSrv.Close()

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		repositoryID := 2
		timeNow := time.Now().UTC()
		localRepoInbox := u.JoinPath(fmt.Sprintf("/api/v1/activitypub/repository-id/%d/inbox", repositoryID)).String()

		ctx, _ := contexttest.MockAPIContext(t, localRepoInbox)
		cf, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		c, err := cf.WithKeysDirect(ctx, mock.Persons[0].PrivKey,
			mock.Persons[0].KeyID(federatedSrv.URL), nil)
		require.NoError(t, err)

		activity1 := fmt.Appendf(nil,
			`{"type":"Like",`+
				`"startTime":"%s",`+
				`"actor":"%s/api/v1/activitypub/user-id/15",`+
				`"object":"%s"}`,
			timeNow.Format(time.RFC3339),
			federatedSrv.URL, u.JoinPath(fmt.Sprintf("/api/v1/activitypub/repository-id/%d", repositoryID)).String())
		t.Logf("activity: %s", activity1)
		resp, err := c.Post(activity1, localRepoInbox)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		federationHost := unittest.AssertExistsAndLoadBean(t, &forgefed.FederationHost{HostFqdn: "127.0.0.1"})
		federatedUser := unittest.AssertExistsAndLoadBean(t, &user.FederatedUser{ExternalID: "15", FederationHostID: federationHost.ID})
		unittest.AssertExistsAndLoadBean(t, &user.User{ID: federatedUser.UserID})

		// A like activity by a different user of the same federated host.
		activity2 := fmt.Appendf(nil,
			`{"type":"Like",`+
				`"startTime":"%s",`+
				`"actor":"%s/api/v1/activitypub/user-id/30",`+
				`"object":"%s"}`,
			// Make sure this activity happens later then the one before
			timeNow.Add(time.Second).Format(time.RFC3339),
			federatedSrv.URL, u.JoinPath(fmt.Sprintf("/api/v1/activitypub/repository-id/%d", repositoryID)).String())
		t.Logf("activity: %s", activity2)
		resp, err = c.Post(activity2, localRepoInbox)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		federatedUser = unittest.AssertExistsAndLoadBean(t, &user.FederatedUser{ExternalID: "30", FederationHostID: federationHost.ID})
		unittest.AssertExistsAndLoadBean(t, &user.User{ID: federatedUser.UserID})

		// The same user sends another like activity
		otherRepositoryID := 3
		otherRepoInboxURL := u.JoinPath(fmt.Sprintf("/api/v1/activitypub/repository-id/%d/inbox", otherRepositoryID)).String()
		activity3 := fmt.Appendf(nil,
			`{"type":"Like",`+
				`"startTime":"%s",`+
				`"actor":"%s/api/v1/activitypub/user-id/30",`+
				`"object":"%s"}`,
			// Make sure this activity happens later then the ones before
			timeNow.Add(time.Second*2).Format(time.RFC3339),
			federatedSrv.URL, u.JoinPath(fmt.Sprintf("/api/v1/activitypub/repository-id/%d", otherRepositoryID)).String())
		t.Logf("activity: %s", activity3)
		resp, err = c.Post(activity3, otherRepoInboxURL)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		federatedUser = unittest.AssertExistsAndLoadBean(t, &user.FederatedUser{ExternalID: "30", FederationHostID: federationHost.ID})
		unittest.AssertExistsAndLoadBean(t, &user.User{ID: federatedUser.UserID})

		// Replay activity2.
		resp, err = c.Post(activity2, localRepoInbox)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
	})
}

func TestActivityPubRepositoryInboxInvalid(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.SignatureEnforced, false)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		apServerActor := user.NewAPServerActor()
		repositoryID := 2
		localRepo2Inbox := u.JoinPath(fmt.Sprintf("/api/v1/activitypub/repository-id/%d/inbox", repositoryID)).String()

		ctx, _ := contexttest.MockAPIContext(t, localRepo2Inbox)
		cf, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		c, err := cf.WithKeys(ctx, apServerActor, apServerActor.KeyID(), nil)
		require.NoError(t, err)

		activity := []byte(`{"type":"Wrong"}`)
		resp, err := c.Post(activity, localRepo2Inbox)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotAcceptable, resp.StatusCode)
	})
}
