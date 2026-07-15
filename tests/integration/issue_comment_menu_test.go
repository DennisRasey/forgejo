// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/tests"
)

func testIssueCommentContextMenuItems(t *testing.T, page *HTMLDoc, quoteReference, editDelete, report bool) {
	// Copy link and Copy Markdown buttons are always available
	page.AssertElement(t, ".comment:nth-child(4) button[data-clipboard-text-type='url']", true)
	page.AssertElement(t, ".comment:nth-child(4) button[data-clipboard-target$='-raw']", true)

	// Buttons Quote reply and Reference in a new issue are only available to signed in users
	page.AssertElement(t, ".comment:nth-child(4) button.quote-reply", quoteReference)
	page.AssertElement(t, ".comment:nth-child(4) button.reference-issue", quoteReference)

	// Buttons Edit and Delete are available to users with write access to issues
	page.AssertElement(t, ".comment:nth-child(4) button.edit-content", editDelete)
	page.AssertElement(t, ".comment:nth-child(4) button.delete-comment", editDelete)

	// Delete button should never be available in the top comment
	page.AssertElement(t, ".first.comment button.delete-comment", false)

	// Report button is available to logged in users when instance moderation is enabled
	page.AssertElement(t, ".first.comment a[href^='/report_abuse?type=issue&id=']", report)
	page.AssertElement(t, ".comment:nth-child(4) a[href^='/report_abuse?type=comment&id=']", report)
}

// TestIssueCommentContextMenu verifies go template logic of that decides which
// context menu items should and should not be available
// Note: this test doesn't cover many cases and can be extended
func TestIssueCommentContextMenu(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := loginUser(t, "user2")

	t.Run("Default conditions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Unauthenticated
		page := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1"), http.StatusOK).Body)
		testIssueCommentContextMenuItems(t, page, false, false, false)

		// Authenticated as repo owner
		page = NewHTMLParser(t, user2.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1"), http.StatusOK).Body)
		testIssueCommentContextMenuItems(t, page, true, true, false)
	})

	t.Run("With moderation enabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		defer test.MockVariableValue(&setting.Moderation.Enabled, true)()

		// Unauthenticated
		page := NewHTMLParser(t, MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1"), http.StatusOK).Body)
		testIssueCommentContextMenuItems(t, page, false, false, false)

		// Authenticated as repo owner
		page = NewHTMLParser(t, user2.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1"), http.StatusOK).Body)
		testIssueCommentContextMenuItems(t, page, true, true, true)
	})
}
