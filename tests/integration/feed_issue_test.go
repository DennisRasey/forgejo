// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"encoding/xml"
	"net/http"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeedIssue(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestFeed")()
	defer tests.PrepareTestEnv(t)()

	t.Run("Issue", func(t *testing.T) {
		t.Run("Atom", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/user2/repo1/issues/1.atom")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<feed xmlns="http://www.w3.org/2005/Atom"`)
		})

		t.Run("RSS", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/user2/repo1/issues/1.rss")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<rss version="2.0"`)

			var rss RSS
			err := xml.Unmarshal(resp.Body.Bytes(), &rss)
			require.NoError(t, err)
			assert.Contains(t, rss.Channel.Link, "/user2")
			assert.NotEmpty(t, rss.Channel.Items)
			assert.Equal(t, "Issue user2/repo1#1: issue1", rss.Channel.Title)
			assert.Equal(t, "Updates on issue user2/repo1#1 by User One", rss.Channel.Description)
			assert.Regexp(t, `http://localhost:\d+/user2/repo1/issues/1#issuecomment-\d+`, rss.Channel.Items[0].Link)
			assert.NotEmpty(t, rss.Channel.PubDate)
		})
	})
}
