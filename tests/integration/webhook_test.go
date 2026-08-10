// Copyright 2024 The Forgejo Authors c/o Codeberg e.V.. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	webhook_model "forgejo.org/models/webhook"
	"forgejo.org/modules/git"
	"forgejo.org/modules/gitrepo"
	"forgejo.org/modules/json"
	webhook_module "forgejo.org/modules/webhook"
	"forgejo.org/services/release"
	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookPayloadRef(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, giteaURL *url.URL) {
		w := unittest.AssertExistsAndLoadBean(t, &webhook_model.Webhook{ID: 1})
		w.HookEvent = &webhook_module.HookEvent{
			SendEverything: true,
		}
		require.NoError(t, w.UpdateEvent())
		require.NoError(t, webhook_model.UpdateWebhook(db.DefaultContext, w))

		hookTasks := retrieveHookTasks(t, w.ID, true)
		hookTasksLenBefore := len(hookTasks)

		session := loginUser(t, "user2")
		// create new branch
		req := NewRequestWithValues(t, "POST", "user2/repo1/branches/_new/branch/master", map[string]string{
			"new_branch_name": "arbre",
			"create_tag":      "false",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		// delete the created branch
		req = NewRequestWithValues(t, "POST", "user2/repo1/branches/delete", map[string]string{
			"name": "arbre",
		})
		session.MakeRequest(t, req, http.StatusOK)

		// check the newly created hooktasks
		hookTasks = retrieveHookTasks(t, w.ID, false)
		expected := map[webhook_module.HookEventType]bool{
			webhook_module.HookEventCreate: true,
			webhook_module.HookEventPush:   true, // the branch creation also creates a push event
			webhook_module.HookEventDelete: true,
		}
		for _, hookTask := range hookTasks[:len(hookTasks)-hookTasksLenBefore] {
			if !expected[hookTask.EventType] {
				t.Errorf("unexpected (or duplicated) event %q", hookTask.EventType)
			}

			var payload struct {
				Ref string `json:"ref"`
			}
			require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payload))
			assert.Equal(t, "refs/heads/arbre", payload.Ref, "unexpected ref for %q event", hookTask.EventType)
			delete(expected, hookTask.EventType)
		}
		assert.Empty(t, expected)
	})
}

func TestWebhookReleaseEvents(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	w := unittest.AssertExistsAndLoadBean(t, &webhook_model.Webhook{
		ID:     1,
		RepoID: repo.ID,
	})
	w.HookEvent = &webhook_module.HookEvent{
		SendEverything: true,
	}
	require.NoError(t, w.UpdateEvent())
	require.NoError(t, webhook_model.UpdateWebhook(db.DefaultContext, w))

	hookTasks := retrieveHookTasks(t, w.ID, true)

	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
	require.NoError(t, err)
	defer gitRepo.Close()

	t.Run("CreateRelease", func(t *testing.T) {
		require.NoError(t, release.CreateRelease(gitRepo, &repo_model.Release{
			RepoID:       repo.ID,
			Repo:         repo,
			PublisherID:  user.ID,
			Publisher:    user,
			TagName:      "v1.1.1",
			Target:       "master",
			Title:        "v1.1.1 is released",
			Note:         "v1.1.1 is released",
			IsDraft:      false,
			IsPrerelease: false,
			IsTag:        false,
		}, "", nil))

		// check the newly created hooktasks
		hookTasksLenBefore := len(hookTasks)
		hookTasks = retrieveHookTasks(t, w.ID, false)

		checkHookTasks(t, map[webhook_module.HookEventType]string{
			webhook_module.HookEventRelease: "published",
			webhook_module.HookEventCreate:  "", // a tag was created as well
			webhook_module.HookEventPush:    "", // the tag creation also means a push event
		}, hookTasks[:len(hookTasks)-hookTasksLenBefore])

		t.Run("UpdateRelease", func(t *testing.T) {
			rel := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{RepoID: repo.ID, TagName: "v1.1.1"})
			require.NoError(t, release.UpdateRelease(db.DefaultContext, user, gitRepo, rel, false, nil))

			// check the newly created hooktasks
			hookTasksLenBefore := len(hookTasks)
			hookTasks = retrieveHookTasks(t, w.ID, false)

			checkHookTasks(t, map[webhook_module.HookEventType]string{
				webhook_module.HookEventRelease: "updated",
			}, hookTasks[:len(hookTasks)-hookTasksLenBefore])
		})
	})

	t.Run("CreateNewTag", func(t *testing.T) {
		require.NoError(t, release.CreateNewTag(
			db.DefaultContext,
			user,
			repo,
			"master",
			"v1.1.2",
			"v1.1.2 is tagged",
		))

		// check the newly created hooktasks
		hookTasksLenBefore := len(hookTasks)
		hookTasks = retrieveHookTasks(t, w.ID, false)

		checkHookTasks(t, map[webhook_module.HookEventType]string{
			webhook_module.HookEventCreate: "", // tag was created as well
			webhook_module.HookEventPush:   "", // the tag creation also means a push event
		}, hookTasks[:len(hookTasks)-hookTasksLenBefore])

		t.Run("UpdateRelease", func(t *testing.T) {
			rel := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{RepoID: repo.ID, TagName: "v1.1.2"})
			require.NoError(t, release.UpdateRelease(db.DefaultContext, user, gitRepo, rel, true, nil))

			// check the newly created hooktasks
			hookTasksLenBefore := len(hookTasks)
			hookTasks = retrieveHookTasks(t, w.ID, false)

			checkHookTasks(t, map[webhook_module.HookEventType]string{
				webhook_module.HookEventRelease: "published",
			}, hookTasks[:len(hookTasks)-hookTasksLenBefore])
		})
	})
}

func checkHookTasks(t *testing.T, expectedActions map[webhook_module.HookEventType]string, hookTasks []*webhook_model.HookTask) {
	t.Helper()
	for _, hookTask := range hookTasks {
		expectedAction, ok := expectedActions[hookTask.EventType]
		if !ok {
			t.Errorf("unexpected (or duplicated) event %q", hookTask.EventType)
		}
		var payload struct {
			Action string `json:"action"`
		}
		require.NoError(t, json.Unmarshal([]byte(hookTask.PayloadContent), &payload))
		assert.Equal(t, expectedAction, payload.Action, "unexpected action for %q event", hookTask.EventType)
		delete(expectedActions, hookTask.EventType)
	}
	assert.Empty(t, expectedActions)
}

func TestWebHooksDelivery(t *testing.T) {
	requests := make(chan *http.Request, 5)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// the body can't be read when the handler returns
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		requests <- req
	}))
	t.Cleanup(receiver.Close)
	maxPayloadDelay := 1 * time.Second
	receiveWebHook := func(t *testing.T) *http.Request {
		select {
		case req := <-requests:
			return req
		case <-time.After(maxPayloadDelay):
			t.Fatalf("no webhook received within %v", maxPayloadDelay)
			return nil
		}
	}

	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		owner := forgery.CreateUser(t, nil)
		session := loginUser(t, owner.Name)
		repo := forgery.CreateRepository(t, owner, nil)
		repoURL := repo.HTMLURL() + "/"

		matrixURL := receiver.URL + "/matrix"
		resp := session.MakeRequest(t, NewRequestWithValues(t, "POST", repoURL+"settings/hooks/matrix/new", map[string]string{
			"homeserver_url": matrixURL,
			"access_token":   "123456",
			"room_id":        "123",
			"events":         "send_everything",
			"active":         "1",
		}), http.StatusSeeOther)
		assertHasFlashMessages(t, resp, "success")

		t.Run("Issue/comments", func(t *testing.T) {
			session.MakeRequest(t, NewRequestWithValues(t, "POST", repoURL+"issues/new", map[string]string{
				"title":   "First Issue",
				"content": "wait for it",
			}), http.StatusOK)

			req := receiveWebHook(t)
			assert.True(t, strings.HasPrefix(req.URL.Path, "/matrix/_matrix/client/v3/rooms/123/send/m.room.message/"), req.URL.Path)
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "Issue opened")

			var firstCommentURL, secondCommentURL string
			var firstCommentPayload, secondCommentPayload []byte
			session.MakeRequest(t, NewRequestWithValues(t, "POST", repoURL+"issues/1/comments", map[string]string{
				"content": "eager to see it",
			}), http.StatusOK)
			req = receiveWebHook(t)
			assert.True(t, strings.HasPrefix(req.URL.Path, "/matrix/_matrix/client/v3/rooms/123/send/m.room.message/"), req.URL.Path)
			firstCommentURL = req.URL.Path

			firstCommentPayload, err = io.ReadAll(req.Body)
			require.NoError(t, err)

			session.MakeRequest(t, NewRequestWithValues(t, "POST", repoURL+"issues/1/comments", map[string]string{
				"content": "me too",
			}), http.StatusOK)
			req = receiveWebHook(t)
			assert.True(t, strings.HasPrefix(req.URL.Path, "/matrix/_matrix/client/v3/rooms/123/send/m.room.message/"), req.URL.Path)
			secondCommentURL = req.URL.Path

			secondCommentPayload, err = io.ReadAll(req.Body)
			require.NoError(t, err)

			// even though the body is the same, the second comment
			// should be considered different from the first (stateKey)
			assert.NotEqual(t, firstCommentURL, secondCommentURL)
			assert.Equal(t, string(firstCommentPayload), string(secondCommentPayload))
		})
	})
}
