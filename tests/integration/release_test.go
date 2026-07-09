// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/translation"
	"forgejo.org/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createNewRelease(t *testing.T, session *TestSession, repoURL, tag, title string, preRelease, draft bool) {
	createNewReleaseTarget(t, session, repoURL, tag, title, "master", preRelease, draft)
}

func createNewReleaseTarget(t *testing.T, session *TestSession, repoURL, tag, title, target string, preRelease, draft bool) {
	req := NewRequest(t, "GET", repoURL+"/releases/new")
	resp := session.MakeRequest(t, req, http.StatusOK)
	page := NewHTMLParser(t, resp.Body)

	// Buttons that should be present
	page.AssertElement(t, `form button[name="tag_only"]`, true) // Create tag
	page.AssertElement(t, `form button[name="draft"]`, true)    // Save draft
	assert.Contains(t, page.Find(`form .primary.button`).Text(), "Publish release")

	// Buttons that should not be present
	page.AssertElement(t, `form a.danger.button[data-modal-id="delete-release"]`, false)
	page.AssertElement(t, `form a.button[href$="/releases"]`, false) // Cancel

	link, exists := page.Find("form[action$='/releases/new']").Attr("action")
	assert.True(t, exists, "The template has changed")

	postData := map[string]string{
		"tag_name":   tag,
		"tag_target": target,
		"title":      title,
		"content":    "",
	}
	if preRelease {
		postData["prerelease"] = "on"
	}
	if draft {
		postData["draft"] = "Save Draft"
	}
	req = NewRequestWithValues(t, "POST", link, postData)

	resp = session.MakeRequest(t, req, http.StatusSeeOther)

	test.RedirectURL(resp) // check that redirect URL exists
}

func checkLatestReleaseAndCount(t *testing.T, session *TestSession, repoURL, version, label string, count int) {
	req := NewRequest(t, "GET", repoURL+"/releases")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	labelText := htmlDoc.doc.Find("#release-list > li > .release-title-wrap .label").First().Text()
	assert.Equal(t, label, labelText)
	titleText := htmlDoc.doc.Find("#release-list > li > .release-title-wrap h4 a").First().Text()
	assert.Equal(t, version, titleText)

	// Check release count in the counter on the Release/Tag switch, as well as that the tab is highlighted
	if count < 10 { // Only check values less than 10, should be enough attempts before this test cracks
		// 10 is the pagination limit, but the counter can have more than that
		releaseTab := htmlDoc.doc.Find(".repository.releases .switch a.active.item[href$='/releases']")
		assert.Contains(t, releaseTab.Text(), strconv.Itoa(count)+" release") // Could be "1 release" or "4 releases"
	}

	releaseList := htmlDoc.doc.Find("#release-list > li")
	assert.Equal(t, count, releaseList.Length())
}

func TestViewReleases(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/releases")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewReleasesNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/releases")
	MakeRequest(t, req, http.StatusOK)
}

func TestCreateRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", false, false)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", translation.NewLocale("en-US").TrString("repo.release.stable"), 4)
}

func TestDeleteRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 57, OwnerName: "user2", LowerName: "repo-release"})
	release := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{TagName: "v2.0"})
	assert.False(t, release.IsTag)

	session := loginUser(t, "user2")   // owner user session
	session5 := loginUser(t, "user5")  // different user session; using the ID of a release that does not belong to the repository must fail
	anonSession := emptyTestSession(t) // anonymous session
	otherRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user5", LowerName: "repo4"})

	// can't delete a release by ID from the wrong repository context (otherRepo)
	req := NewRequest(t, "POST", fmt.Sprintf("%s/releases/delete?id=%d", otherRepo.Link(), release.ID))
	session5.MakeRequest(t, req, http.StatusNotFound)

	// can't delete a release that the current user isn't a writer for
	req = NewRequest(t, "POST", fmt.Sprintf("%s/releases/delete?id=%d", repo.Link(), release.ID))
	session5.MakeRequest(t, req, http.StatusNotFound)

	// can't delete a release while anonymous
	req = NewRequest(t, "POST", fmt.Sprintf("%s/releases/delete?id=%d", repo.Link(), release.ID))
	anonSession.MakeRequest(t, req, http.StatusSeeOther) // login redirect

	// can't delete a release by ID from the wrong repository context (otherRepo) as the correct user
	req = NewRequest(t, "POST", fmt.Sprintf("%s/releases/delete?id=%d", otherRepo.Link(), release.ID))
	session.MakeRequest(t, req, http.StatusNotFound)

	// but when everything aligns, we can delete the release
	req = NewRequest(t, "POST", fmt.Sprintf("%s/releases/delete?id=%d", repo.Link(), release.ID))
	session.MakeRequest(t, req, http.StatusOK)
	release = unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: release.ID})

	if assert.True(t, release.IsTag) {
		// can't delete a release by ID from the wrong repository context (otherRepo)
		req = NewRequest(t, "POST", fmt.Sprintf("%s/tags/delete?id=%d", otherRepo.Link(), release.ID))
		session5.MakeRequest(t, req, http.StatusNotFound)

		// can't delete a release that the current user isn't a writer for
		req = NewRequest(t, "POST", fmt.Sprintf("%s/tags/delete?id=%d", repo.Link(), release.ID))
		session5.MakeRequest(t, req, http.StatusNotFound)

		// can't delete a release while anonymous
		req = NewRequest(t, "POST", fmt.Sprintf("%s/tags/delete?id=%d", repo.Link(), release.ID))
		anonSession.MakeRequest(t, req, http.StatusSeeOther) // login redirect

		// can't delete a release by ID from the wrong repository context (otherRepo) as the correct user
		req = NewRequest(t, "POST", fmt.Sprintf("%s/tags/delete?id=%d", otherRepo.Link(), release.ID))
		session.MakeRequest(t, req, http.StatusNotFound)

		// but when everything aligns, we can delete the tag
		req = NewRequest(t, "POST", fmt.Sprintf("%s/tags/delete?id=%d", repo.Link(), release.ID))
		session.MakeRequest(t, req, http.StatusOK)

		unittest.AssertNotExistsBean(t, &repo_model.Release{ID: release.ID})
	}
}

func TestCreateReleasePreRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", true, false)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", translation.NewLocale("en-US").TrString("repo.release.prerelease"), 4)
}

func TestCreateReleaseDraft(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", false, true)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", translation.NewLocale("en-US").TrString("repo.release.draft"), 4)
}

func TestEditRelease(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	page := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/releases/edit/v1.0"), http.StatusOK).Body)

	// Buttons that should be present
	page.AssertElement(t, `form .danger.button[data-modal-id="delete-release"]`, true)
	page.AssertElement(t, `form a.button[href$="/releases"]`, true) // Cancel
	assert.Contains(t, page.Find(`form .primary.button`).Text(), "Update release")

	// Buttons that should not be present
	page.AssertElement(t, `form button[name="draft"]`, false)    // Save draft
	page.AssertElement(t, `form button[name="tag_only"]`, false) // Create tag
}

func TestEditReleaseDraft(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	page := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/releases/edit/draft-release"), http.StatusOK).Body)

	// Buttons that should be present
	page.AssertElement(t, `form a.danger.button[data-modal-id="delete-release"]`, true)
	page.AssertElement(t, `form a.button[href$="/releases"]`, true) // Cancel
	page.AssertElement(t, `form .button[name="draft"]`, true)       // Save draft
	assert.Contains(t, page.Find(`form .primary.button`).Text(), "Publish release")

	// Buttons that should not be present
	page.AssertElement(t, `form button[name="tag_only"]`, false) // Create tag
}

func TestCreateReleasePaging(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	oldAPIDefaultNum := setting.API.DefaultPagingNum
	defer func() {
		setting.API.DefaultPagingNum = oldAPIDefaultNum
	}()
	setting.API.DefaultPagingNum = 10

	session := loginUser(t, "user2")
	// Create enough releases to have paging
	for i := range 12 {
		version := fmt.Sprintf("v0.0.%d", i)
		createNewRelease(t, session, "/user2/repo1", version, version, false, false)
	}
	createNewRelease(t, session, "/user2/repo1", "v0.0.12", "v0.0.12", false, true)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.12", translation.NewLocale("en-US").TrString("repo.release.draft"), 10)

	// Check that user4 does not see draft and still see 10 latest releases
	session2 := loginUser(t, "user4")
	checkLatestReleaseAndCount(t, session2, "/user2/repo1", "v0.0.11", translation.NewLocale("en-US").TrString("repo.release.stable"), 10)
}

func TestViewReleaseListNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 57, OwnerName: "user2", LowerName: "repo-release"})

	link := repo.Link() + "/releases"

	req := NewRequest(t, "GET", link)
	rsp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("ul#release-list > li")
	assert.Equal(t, 5, releases.Length())

	links := make([]string, 0, 5)
	commitsToMain := make([]string, 0, 5)
	releases.Each(func(i int, s *goquery.Selection) {
		link, exist := s.Find(".release-title-wrap h4 a").Attr("href")
		if !exist {
			return
		}
		links = append(links, link)

		commitsToMain = append(commitsToMain, s.Find(".ahead > a").Text())
	})

	assert.Equal(t, []string{
		"/user2/repo-release/releases/tag/empty-target-branch",
		"/user2/repo-release/releases/tag/non-existing-target-branch",
		"/user2/repo-release/releases/tag/v2.0",
		"/user2/repo-release/releases/tag/v1.1",
		"/user2/repo-release/releases/tag/v1.0",
	}, links)
	assert.Equal(t, []string{
		"1 commits", // like v1.1
		"1 commits", // like v1.1
		"0 commits",
		"1 commits", // should be 3 commits ahead and 2 commits behind, but not implemented yet
		"3 commits",
	}, commitsToMain)
}

func TestViewSingleReleaseNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo-release/releases/tag/v1.0")
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	// check the "number of commits to main since this release"
	releaseList := htmlDoc.doc.Find("#release-list .ahead > a")
	assert.Equal(t, 1, releaseList.Length())
	assert.Equal(t, "3 commits", releaseList.First().Text())
}

func TestViewReleaseListLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	link := repo.Link() + "/releases"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("ul#release-list > li")
	assert.Equal(t, 3, releases.Length())

	links := make([]string, 0, 5)
	releases.Each(func(i int, s *goquery.Selection) {
		link, exist := s.Find(".release-title-wrap h4 a").Attr("href")
		if !exist {
			return
		}
		links = append(links, link)
	})

	assert.Equal(t, []string{
		"/user2/repo1/releases/tag/draft-release",
		"/user2/repo1/releases/tag/v1.0",
		"/user2/repo1/releases/tag/v1.1",
	}, links)
}

func TestViewReleaseListKeyword(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	link := repo.Link() + "/releases?q=testing"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("ul#release-list > li")
	assert.Equal(t, 1, releases.Length())

	links := make([]string, 0, 5)
	releases.Each(func(i int, s *goquery.Selection) {
		link, exist := s.Find(".release-title-wrap h4 a").Attr("href")
		if !exist {
			return
		}
		links = append(links, link)
	})

	assert.Equal(t, []string{
		"/user2/repo1/releases/tag/v1.1",
	}, links)
}

func TestViewReleaseListKeywordNoPagination(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	link := repo.Link() + "/releases?q=testing&limit=1"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("ul#release-list > li")
	assert.Equal(t, 1, releases.Length())

	pagination := htmlDoc.Find("div.pagination")
	assert.Zero(t, pagination.Length())
}

func TestReleaseOnCommit(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewReleaseTarget(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", "65f1bf27bc3bf70f64657658635e66094edbcb4d", false, false)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", translation.NewLocale("en-US").TrString("repo.release.stable"), 4)
}

func TestViewTagsList(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	link := repo.Link() + "/tags"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	tags := htmlDoc.Find(".tag-list tr")
	assert.Equal(t, 3, tags.Length())

	tagNames := make([]string, 0, 5)
	tags.Each(func(i int, s *goquery.Selection) {
		tagNames = append(tagNames, s.Find(".tag a.tw-flex.tw-items-center").Text())
	})

	assert.Equal(t, []string{"v1.0", "delete-tag", "v1.1"}, tagNames)
}

func TestAttachmentTimestamp(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "user2/repo1/releases")
	resp := MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	var timeStamp int64 = 946684800
	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{
		UUID:        "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a20",
		CreatedUnix: timeutil.TimeStamp(timeStamp),
	})

	formattedTime := time.Unix(timeStamp, 0).Format(time.RFC3339)
	htmlDoc.AssertElement(t, fmt.Sprintf("details.download relative-time[datetime='%s']", formattedTime), true)

	session := loginUser(t, "user2")
	req = NewRequest(t, "GET", "user2/repo1/releases/edit/v1.1")
	resp = session.MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, fmt.Sprintf("relative-time[datetime='%s']", formattedTime), true)
}

func TestDownloadReleaseAttachment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	tests.PrepareAttachmentsStorage(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	url := repo.Link() + "/releases/download/v1.1/README.md"

	// user2/repo2 is private and can't be accessed anonymously
	req := NewRequest(t, "GET", url)
	MakeRequest(t, req, http.StatusNotFound)

	// But the owner can access it
	req = NewRequest(t, "GET", url)
	session := loginUser(t, "user2")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestReleaseAttachmentDownloadCounter(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	tests.PrepareAttachmentsStorage(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	session := loginUser(t, "user2")
	zipAttachmentLink := fmt.Sprintf("%s/archive/v1.1.zip", repo.Link())
	gzAttachmentLink := fmt.Sprintf("%s/archive/v1.1.tar.gz", repo.Link())
	counterSelector := "details.download > ul > li:has(a[href='%s']) span"

	// Assert zero downloads initially
	doc := NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/releases", repo.Link())), http.StatusOK).Body)
	zipDownloads := doc.Find(fmt.Sprintf(counterSelector, zipAttachmentLink)).Text()
	gzDownloads := doc.Find(fmt.Sprintf(counterSelector, gzAttachmentLink)).Text()
	assert.Contains(t, zipDownloads, "0 downloads")
	assert.Contains(t, gzDownloads, "0 downloads")

	// Generate downloads
	session.MakeRequest(t, NewRequest(t, "GET", zipAttachmentLink), http.StatusOK)
	session.MakeRequest(t, NewRequest(t, "GET", gzAttachmentLink), http.StatusOK)
	session.MakeRequest(t, NewRequest(t, "GET", gzAttachmentLink), http.StatusOK)

	// Check the new numbers
	doc = NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/releases", repo.Link())), http.StatusOK).Body)
	zipDownloads = doc.Find(fmt.Sprintf(counterSelector, zipAttachmentLink)).Text()
	gzDownloads = doc.Find(fmt.Sprintf(counterSelector, gzAttachmentLink)).Text()
	assert.Contains(t, zipDownloads, "1 download")
	assert.Contains(t, gzDownloads, "2 downloads")
}

func TestReleaseHideArchiveLinksUI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	release := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{TagName: "v2.0"})

	require.NoError(t, release.LoadAttributes(db.DefaultContext))

	session := loginUser(t, release.Repo.OwnerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	zipURL := fmt.Sprintf("%s/archive/%s.zip", release.Repo.Link(), release.TagName)
	tarGzURL := fmt.Sprintf("%s/archive/%s.tar.gz", release.Repo.Link(), release.TagName)

	resp := session.MakeRequest(t, NewRequest(t, "GET", release.HTMLURL()), http.StatusOK)
	body := resp.Body.String()
	assert.Contains(t, body, zipURL)
	assert.Contains(t, body, tarGzURL)

	hideArchiveLinks := true

	req := NewRequestWithJSON(t, "PATCH", release.APIURL(), &api.EditReleaseOption{
		HideArchiveLinks: &hideArchiveLinks,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	resp = session.MakeRequest(t, NewRequest(t, "GET", release.HTMLURL()), http.StatusOK)
	body = resp.Body.String()
	assert.NotContains(t, body, zipURL)
	assert.NotContains(t, body, tarGzURL)
}
