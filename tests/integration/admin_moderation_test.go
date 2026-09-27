// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"strings"
	"testing"

	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/routers"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

func testReportDetails(t *testing.T, htmlDoc *HTMLDoc, reportID, contentIcon, contentRef, contentURL, category, reportsNo string, hasMoreActions bool) {
	// Check icon octicon
	icon := htmlDoc.Find("#report-" + reportID + " svg." + contentIcon)
	assert.Equal(t, 1, icon.Length())

	// Check content reference and URL
	title := htmlDoc.Find("#report-" + reportID + " .flex-item-main .flex-item-title a")
	if len(contentURL) == 0 {
		// No URL means that the content was already deleted, so we should not find the anchor element.
		assert.Zero(t, title.Length())
		// Instead we should find an emphasis element.
		title = htmlDoc.Find("#report-" + reportID + " .flex-item-main .flex-item-title em")
		assert.Equal(t, 1, title.Length())
		assert.Equal(t, contentRef, title.Text())
	} else {
		assert.Equal(t, 1, title.Length())
		assert.Equal(t, contentRef, title.Text())

		href, exists := title.Attr("href")
		assert.True(t, exists)
		assert.Equal(t, contentURL, href)
	}

	// Check category
	cat := htmlDoc.Find("#report-" + reportID + " .flex-item-main .flex-items-inline .item:nth-child(3)")
	assert.Equal(t, 1, cat.Length())
	assert.Equal(t, category, strings.TrimSpace(cat.Text()))

	// Check number of reports for the same content
	count := htmlDoc.Find("#report-" + reportID + " a span")
	assert.Equal(t, 1, count.Length())
	assert.Equal(t, reportsNo, count.Text())

	// Check 'More actions' (⋯) dropdown
	htmlDoc.AssertElement(t, "#report-"+reportID+" > .flex-item-trailing .button-sequence .dialog-dropdown", hasMoreActions)
}

func TestAdminModerationViewReports(t *testing.T) {
	defer unittest.OverrideFixtures("tests/integration/fixtures/TestAdminModerationViewReports")()
	defer tests.PrepareTestEnv(t)()

	t.Run("Moderation enabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Moderation.Enabled, true)()
		defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

		t.Run("Anonymous user", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/admin/moderation/reports")
			MakeRequest(t, req, http.StatusSeeOther)
			req = NewRequest(t, "GET", "/admin/moderation/reports/type/1/id/1002")
			MakeRequest(t, req, http.StatusSeeOther)
		})

		t.Run("Normal user", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			session := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/admin/moderation/reports")
			session.MakeRequest(t, req, http.StatusForbidden)
			req = NewRequest(t, "GET", "/admin/moderation/reports/type/1/id/1002")
			session.MakeRequest(t, req, http.StatusForbidden)
		})

		t.Run("Admin user", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			session := loginUser(t, "user1")
			req := NewRequest(t, "GET", "/admin/moderation/reports")
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)

			// Check how many reports are being displayed.
			// Reports linked to the same content (type and id) should be grouped; therefore we should see only 11 instead of 14.
			reports := htmlDoc.Find(".admin-setting-content .flex-list .flex-item.report")
			assert.Equal(t, 11, reports.Length())

			// Check details for shown reports.
			testReportDetails(t, htmlDoc, "1", "octicon-person", "@SPAM-services", "/SPAM-services", "Illegal content", "1", true)
			testReportDetails(t, htmlDoc, "2", "octicon-repo", "SPAM-services/spammer-Tools", "/SPAM-services/spammer-Tools", "Illegal content", "1", true)
			testReportDetails(t, htmlDoc, "3", "octicon-issue-opened", "SPAM-services/spammer-Tools#1", "/SPAM-services/spammer-Tools/issues/1", "Spam", "1", true)
			// #4 is combined with #7 and #9
			testReportDetails(t, htmlDoc, "4", "octicon-person", "@spammer01", "/spammer01", "Spam", "3", true)
			// #5 is combined with #6
			testReportDetails(t, htmlDoc, "5", "octicon-comment", "contributor/first/issues/1#issuecomment-1001", "/contributor/first/issues/1#issuecomment-1001", "Malware", "2", true)
			testReportDetails(t, htmlDoc, "8", "octicon-issue-opened", "contributor/first#1", "/contributor/first/issues/1", "Other violations of platform rules", "1", true)
			// #10 is for a Ghost user
			testReportDetails(t, htmlDoc, "10", "octicon-person", "Reported content with type 1 and id 9999 no longer exists", "", "Other violations of platform rules", "1", false)
			// #11 is for a comment who's poster was deleted
			testReportDetails(t, htmlDoc, "11", "octicon-comment", "contributor/first/issues/1#issuecomment-1003", "/contributor/first/issues/1#issuecomment-1003", "Spam", "1", true)
			// #12 is for a comment that was deleted but its poster still exists
			testReportDetails(t, htmlDoc, "12", "octicon-comment", "Reported content with type 4 and id 9991 no longer exists", "", "Other violations of platform rules", "1", false)
			// #13 is for a comment that was deleted and the poster was also deleted
			testReportDetails(t, htmlDoc, "13", "octicon-comment", "Reported content with type 4 and id 9992 no longer exists", "", "Spam", "1", false)
			// #14 is for a issue that was deleted but its poster still exists
			testReportDetails(t, htmlDoc, "14", "octicon-issue-opened", "Reported content with type 3 and id 9991 no longer exists", "", "Spam", "1", false)

			t.Run("reports details page", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				// ## There are 3 open reports linked to user 1002.
				req = NewRequest(t, "GET", "/admin/moderation/reports/type/1/id/1002")
				resp = session.MakeRequest(t, req, http.StatusOK)
				htmlDoc = NewHTMLParser(t, resp.Body)

				// Check the title (content reference) and corresponding URL.
				title := htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title a")
				assert.Equal(t, 1, title.Length())
				assert.Equal(t, "spammer01", title.Text())
				href, exists := title.Attr("href")
				assert.True(t, exists)
				assert.Equal(t, "/spammer01", href)

				// Check how many reports are being displayed for user 1002.
				reports = htmlDoc.Find(".admin-setting-content .flex-list .flex-item")
				assert.Equal(t, 3, reports.Length())

				// ## Poster of comment 1003 was deleted; make sure the details page is still rendered correctly.
				req = NewRequest(t, "GET", "/admin/moderation/reports/type/4/id/1003")
				resp = session.MakeRequest(t, req, http.StatusOK)
				htmlDoc = NewHTMLParser(t, resp.Body)

				// Check the title (content reference) and corresponding URL.
				title = htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title a")
				assert.Equal(t, 1, title.Length())
				assert.Equal(t, "/contributor/first/issues/1#issuecomment-1003", title.Text())
				href, exists = title.Attr("href")
				assert.True(t, exists)
				assert.Equal(t, "/contributor/first/issues/1#issuecomment-1003", href)

				// ## Comment 9991 was deleted but its poster still exists.
				req = NewRequest(t, "GET", "/admin/moderation/reports/type/4/id/9991")
				resp = session.MakeRequest(t, req, http.StatusOK)
				htmlDoc = NewHTMLParser(t, resp.Body)

				// Check the title (content reference); no URL in case of deleted content.
				title = htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title a")
				assert.Equal(t, 0, title.Length()) // no anchor because the reported content was deleted
				title = htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title em")
				assert.Equal(t, 1, title.Length()) // a generic emphasised title is shown for deleted content
				assert.Equal(t, "Reported content with type 4 and id 9991 no longer exists", title.Text())

				// Check shadow copies for deleted comment when poster still exists.
				reports := htmlDoc.Find(".admin-setting-content .flex-list .flex-item")
				assert.Equal(t, 1, reports.Length()) // there is only one report for this content
				shadowCopyRows := htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr")
				assert.Equal(t, 6, shadowCopyRows.Length()) // a shadow copy was created and it has 6 fields
				posterRowCells := htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr:first-of-type td")
				assert.Equal(t, 2, posterRowCells.Length())
				assert.Equal(t, "Poster", posterRowCells.Nodes[0].FirstChild.Data) // in case of comment shadow copies, the first field should be 'Poster'
				// For reports of deleted comments, in case the poster still exists
				// the value of 'Poster' field should contain a link to the poster profile.
				poster := htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr:first-of-type td:last-of-type a")
				assert.Equal(t, 1, poster.Length())
				assert.Equal(t, "grace", poster.Text())
				href, exists = poster.Attr("href")
				assert.True(t, exists)
				assert.Equal(t, "/grace", href)

				// ## Comment 9992 was deleted and its poster was also deleted.
				req = NewRequest(t, "GET", "/admin/moderation/reports/type/4/id/9992")
				resp = session.MakeRequest(t, req, http.StatusOK)
				htmlDoc = NewHTMLParser(t, resp.Body)

				// Check the title (content reference); no URL in case of deleted content.
				title = htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title a")
				assert.Equal(t, 0, title.Length()) // no anchor because the reported content was deleted
				title = htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title em")
				assert.Equal(t, 1, title.Length()) // a generic emphasised title is shown for deleted content
				assert.Equal(t, "Reported content with type 4 and id 9992 no longer exists", title.Text())

				// Check shadow copies for deleted comment when poster was also deleted.
				reports = htmlDoc.Find(".admin-setting-content .flex-list .flex-item")
				assert.Equal(t, 1, reports.Length()) // there is only one report for this content
				shadowCopyRows = htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr")
				assert.Equal(t, 6, shadowCopyRows.Length()) // a shadow copy was created and it has 6 fields
				posterRowCells = htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr:first-of-type td")
				assert.Equal(t, 2, posterRowCells.Length())
				assert.Equal(t, "Poster", posterRowCells.Nodes[0].FirstChild.Data) // in case of comment shadow copies, the first field should be 'Poster'
				// For reports of deleted comments, in case the poster was also deleted
				// the value of 'Poster' field cannot contain a link to the poster profile but only their ID.
				poster = htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr:first-of-type td:last-of-type a")
				assert.Equal(t, 0, poster.Length())
				poster = htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr:first-of-type td:last-of-type")
				assert.Equal(t, 1, poster.Length())
				assert.Equal(t, "9999", poster.Text())

				// ## Issue 9991 was deleted but its poster still exists.
				req = NewRequest(t, "GET", "/admin/moderation/reports/type/3/id/9991")
				resp = session.MakeRequest(t, req, http.StatusOK)
				htmlDoc = NewHTMLParser(t, resp.Body)

				// Check the title (content reference); no URL in case of deleted content.
				title = htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title a")
				assert.Equal(t, 0, title.Length()) // no anchor because the reported content was deleted
				title = htmlDoc.Find(".admin-setting-content .flex-item-main .flex-item-title em")
				assert.Equal(t, 1, title.Length()) // a generic emphasised title is shown for deleted content
				assert.Equal(t, "Reported content with type 3 and id 9991 no longer exists", title.Text())

				// Check shadow copies for deleted issue when poster still exists.
				reports = htmlDoc.Find(".admin-setting-content .flex-list .flex-item")
				assert.Equal(t, 1, reports.Length()) // there is only one report for this content
				shadowCopyRows = htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr")
				assert.Equal(t, 8, shadowCopyRows.Length()) // a shadow copy was created and it has 8 fields
				posterRowCells = htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr:nth-of-type(3) td")
				assert.Equal(t, 2, posterRowCells.Length())
				assert.Equal(t, "Poster", posterRowCells.Nodes[0].FirstChild.Data) // in case of issue shadow copies, the third field should be 'Poster'
				// For reports of deleted issues, in case the poster still exists
				// the value of 'Poster' field should contain a link to the poster profile.
				poster = htmlDoc.Find(".admin-setting-content .flex-list .flex-item:first-of-type details table tr:nth-of-type(3) td:last-of-type a")
				assert.Equal(t, 1, poster.Length())
				assert.Equal(t, "frank", poster.Text())
				href, exists = poster.Attr("href")
				assert.True(t, exists)
				assert.Equal(t, "/frank", href)
			})
		})
	})

	t.Run("Moderation disabled", func(t *testing.T) {
		t.Run("Anonymous user", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/admin/moderation/reports")
			MakeRequest(t, req, http.StatusNotFound)
			req = NewRequest(t, "GET", "/admin/moderation/reports/type/1/id/1002")
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("Normal user", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			session := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/admin/moderation/reports")
			session.MakeRequest(t, req, http.StatusNotFound)
			req = NewRequest(t, "GET", "/admin/moderation/reports/type/1/id/1002")
			session.MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("Admin user", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			session := loginUser(t, "user1")
			req := NewRequest(t, "GET", "/admin/moderation/reports")
			session.MakeRequest(t, req, http.StatusNotFound)
			req = NewRequest(t, "GET", "/admin/moderation/reports/type/1/id/1002")
			session.MakeRequest(t, req, http.StatusNotFound)
		})
	})
}
