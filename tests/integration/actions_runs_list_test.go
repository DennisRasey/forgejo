// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"regexp"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionRunsList(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestActionRunsList")()
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user5/repo4/actions")
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	runTitles := htmlDoc.Find(".run-list .flex-item-title")
	runDescriptions := htmlDoc.Find(".run-list .flex-item-body")

	allWhitespacePattern := regexp.MustCompile(`\s+`)

	assert.Equal(t, 8, runDescriptions.Length())

	dispatchTitle, err := runTitles.Eq(0).Html()
	require.NoError(t, err)
	assert.Contains(t, dispatchTitle, "Running workflow_dispatch <code class=\"inline-code-block\">bar</code>")

	assert.Contains(t, allWhitespacePattern.ReplaceAllString(runDescriptions.Eq(0).Text(), " "),
		"dispatch.yaml #210 - Run of commit dc67f0a1a0 triggered by user29")
	assert.Equal(t, "/user5/repo4/commit/dc67f0a1a0dc2417cd0b1a9f0b95e2e5d91876e9",
		runDescriptions.Eq(0).Find("a").Eq(0).AttrOr("href", ""))
	assert.Equal(t, "/user29",
		runDescriptions.Eq(0).Find("a").Eq(1).AttrOr("href", ""))

	scheduledTitle, err := runTitles.Eq(1).Html()
	require.NoError(t, err)
	assert.Contains(t, scheduledTitle, "A scheduled workflow <code class=\"inline-code-block\">foo</code>")

	assert.Contains(t, allWhitespacePattern.ReplaceAllString(runDescriptions.Eq(1).Text(), " "),
		"scheduled.yaml #209 - Scheduled run of commit 64357baca8")
	assert.Equal(t, "/user5/repo4/commit/64357baca84bfff631e7dfae5a3433b26d005646",
		runDescriptions.Eq(1).Find("a").Eq(0).AttrOr("href", ""))

	pushTitle, err := runTitles.Eq(2).Html()
	require.NoError(t, err)
	assert.Contains(t, pushTitle, "job output <code class=\"inline-code-block\">baz</code>")

	assert.Contains(t, allWhitespacePattern.ReplaceAllString(runDescriptions.Eq(2).Text(), " "),
		"test.yaml #192 - Commit c2d72f5484 pushed by user1")
	assert.Equal(t, "/user5/repo4/commit/c2d72f548424103f01ee1dc02889c1e2bff816b0",
		runDescriptions.Eq(2).Find("a").Eq(0).AttrOr("href", ""))
	assert.Equal(t, "/user1",
		runDescriptions.Eq(2).Find("a").Eq(1).AttrOr("href", ""))
}
