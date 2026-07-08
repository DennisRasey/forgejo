// Copyright 2022, 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"forgejo.org/models/activities"
	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/activitypub"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/routers"
	"forgejo.org/services/contexttest"
	"forgejo.org/services/federation"
	"forgejo.org/tests/forgery"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityPubPersonInboxNoteFromDistant(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	federation.Init()

	mock := test.NewFederationServerMock()
	federatedSrv := mock.DistantServer(t)
	defer federatedSrv.Close()

	onApplicationRun(t, func(t *testing.T, localUrl *url.URL) {
		defer test.MockVariableValue(&setting.AppURL, localUrl.String())()

		distantURL := federatedSrv.URL
		distantUser15URL := fmt.Sprintf("%s/api/v1/activitypub/user-id/15", distantURL)

		localUser2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		localUser2URL := localUrl.JoinPath("/api/v1/activitypub/user-id/2").String()
		localUser2Inbox := localUrl.JoinPath("/api/v1/activitypub/user-id/2/inbox").String()
		localSession2 := loginUser(t, localUser2.LoginName)
		localSecssion2Token := getTokenForLoggedInUser(t, localSession2, auth_model.AccessTokenScopeWriteUser)

		// view own empty feed on web UI
		feedPage := NewHTMLParser(t, localSession2.MakeRequest(t, NewRequest(t, "GET", "/user2?tab=feed"), http.StatusOK).Body)
		feedPage.AssertElement(t, "#empty-ap-feed", true)

		// follow (local follows distant)
		req := NewRequestWithJSON(t, "POST",
			"/api/v1/user/activitypub/follow",
			&structs.APRemoteFollowOption{
				Target: distantUser15URL,
			}).
			AddTokenAuth(localSecssion2Token)
		MakeRequest(t, req, http.StatusNoContent)

		// send note (distant -> local)
		distantNoteURL := fmt.Sprintf("%s/api/v1/activitypub/note/104", distantURL)

		userActivity := fmt.Appendf(
			[]byte{},
			`{"type":"Create",`+
				`"actor":"%s",`+
				`"to": ["https://www.w3.org/ns/activitystreams#Public"],`+
				`"cc": ["%s"],`+
				`"object": {"type":"Note","content":"The Content!",`+
				`"url":"%s"}}`,
			distantUser15URL,
			localUser2URL,
			distantNoteURL,
		)

		ctx, _ := contexttest.MockAPIContext(t, localUser2Inbox)
		cf, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		c, err := cf.WithKeysDirect(ctx, mock.ApActor.PrivKey, mock.ApActor.KeyID(federatedSrv.URL), nil)
		require.NoError(t, err)

		resp, err := c.Post(userActivity, localUser2Inbox)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// check whether user activity exists in local instance
		unittest.AssertExistsAndLoadBean(t, &activities.FederatedUserActivity{NoteURL: distantNoteURL})

		// view own non-empty feed on web UI
		feedPage = NewHTMLParser(t, localSession2.MakeRequest(t, NewRequest(t, "GET", "/user2?tab=feed"), http.StatusOK).Body)
		feedPage.AssertElement(t, "#empty-ap-feed", false)
	})
}

func TestActivityPubPersonInboxNoteToDistant(t *testing.T) {
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&setting.Federation.SignatureEnforced, true)()
	defer test.MockVariableValue(&setting.Federation.InsecureAllowInvalidHosts, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	federation.Init()

	mock := test.NewFederationServerMock()
	federatedSrv := mock.DistantServer(t)
	defer federatedSrv.Close()

	onApplicationRun(t, func(t *testing.T, localUrl *url.URL) {
		defer test.MockVariableValue(&setting.AppURL, localUrl.String())()

		distantURL := federatedSrv.URL
		distantUser15URL := fmt.Sprintf("%s/api/v1/activitypub/user-id/15", distantURL)

		localUser2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		localUser2URL := localUrl.JoinPath("/api/v1/activitypub/user-id/2").String()
		localUser2Inbox := fmt.Sprintf("%v/inbox", localUser2URL)
		localSession2 := loginUser(t, localUser2.LoginName)
		localSecssion2Token := getTokenForLoggedInUser(t, localSession2, auth_model.AccessTokenScopeWriteIssue)

		repo := forgery.CreateRepository(t, localUser2, nil)

		// follow (distant follows local)
		followActivity := fmt.Appendf(nil,
			`{"type":"Follow",`+
				`"actor":"%s",`+
				`"object":"%s"}`,
			distantUser15URL,
			localUser2URL,
		)

		ctx, _ := contexttest.MockAPIContext(t, localUser2Inbox)
		cf, err := activitypub.NewClientFactoryWithTimeout(60 * time.Second)
		require.NoError(t, err)

		c, err := cf.WithKeysDirect(ctx, mock.ApActor.PrivKey, mock.ApActor.KeyID(federatedSrv.URL), nil)
		require.NoError(t, err)

		resp, err := c.Post(followActivity, localUser2Inbox)
		require.NoError(t, err)

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		// local action which triggers a user activity
		IssueURL := fmt.Sprintf("/api/v1/repos/%s/issues?state=all", repo.FullName())
		req := NewRequestWithJSON(t, "POST", IssueURL, &structs.CreateIssueOption{
			Title: "ActivityFeed test",
			Body:  "Nothing to see here!",
		}).AddTokenAuth(localSecssion2Token)
		MakeRequest(t, req, http.StatusCreated)

		// distant request outbox
		localUser2Outbox := fmt.Sprintf("%v/outbox", localUser2URL)
		resp, err = c.Get(localUser2Outbox)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// ID of create activity and note delivered to distant
		activity, err := ap.UnmarshalJSON([]byte(mock.LastPost))
		require.NoError(t, err)

		createNote := activity.(*ap.Create)
		localUser2ActivityNote, err := createNote.Object.GetID().URL()
		require.NoError(t, err)

		localUser2Activity, err := createNote.GetID().URL()
		require.NoError(t, err)

		resp, err = c.Get(localUser2ActivityNote.String())
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resp, err = c.Get(localUser2Activity.String())
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// check for activity on distant inbox
		assert.Contains(t, mock.LastPost, "user2</a> opened issue")
	})
}
