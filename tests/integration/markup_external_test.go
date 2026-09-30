// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"forgejo.org/models/unittest"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/git"
	"forgejo.org/modules/setting"
	files_service "forgejo.org/services/repository/files"
	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalMarkupRenderer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	if !setting.Database.Type.IsSQLite3() {
		t.Skip()
		return
	}

	const repoURL = "user30/renderer"
	req := NewRequest(t, "GET", repoURL+"/src/branch/master/README.html")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header()["Content-Type"][0])

	bs, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	doc := NewHTMLParser(t, bytes.NewBuffer(bs))
	div := doc.Find("div.file-view")
	data, err := div.Html()
	require.NoError(t, err)
	assert.Equal(t, "<div>\n\ttest external renderer\n</div>", strings.TrimSpace(data))
}

func TestExternalMarkupRendererWithCompoundFileExtensions(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		testCases := []struct {
			path                      string
			shouldUseExternalRenderer bool
			expectedRenderedContent   string
		}{
			{"foo.abc.def.ghi.jkl.md", true, ".def.ghi.jkl.md renderer used"},
			{"foo.def.ghi.jkl.md", true, ".def.ghi.jkl.md renderer used"},
			{"foo.ghi.jkl.md", true, ".ghi.jkl.md renderer used"},
			{"foo.jkl.md", false, "foo"},
			{"foo.md", false, "foo"},
		}

		changeRepoFiles := []*files_service.ChangeRepoFile{}
		for _, testCase := range testCases {
			changeRepoFiles = append(changeRepoFiles,
				&files_service.ChangeRepoFile{
					Operation:     "create",
					TreePath:      testCase.path,
					ContentReader: strings.NewReader("foo"),
				},
			)
		}

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		repo := forgery.CreateRepository(t, user, nil)

		_, err := files_service.ChangeRepoFiles(git.DefaultContext, repo, user, &files_service.ChangeRepoFilesOptions{
			Files:   changeRepoFiles,
			Message: "add files",
			Author: &files_service.IdentityOptions{
				Name:  user.Name,
				Email: user.Email,
			},
			Committer: &files_service.IdentityOptions{
				Name:  user.Name,
				Email: user.Email,
			},
			Dates: &files_service.CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		})
		require.NoError(t, err)

		for _, testCase := range testCases {
			t.Run(testCase.path, func(t *testing.T) {
				req := NewRequestf(t, "GET", "%s/src/branch/%s/%s", repo.HTMLURL(), repo.DefaultBranch, testCase.path)
				resp := MakeRequest(t, req, http.StatusOK)
				require.Equal(t, "text/html; charset=utf-8", resp.Header()["Content-Type"][0])

				doc := NewHTMLParser(t, resp.Body)
				var query string
				if testCase.shouldUseExternalRenderer {
					query = "div.file-view"
				} else {
					query = "div.file-view p"
				}
				p := doc.Find(query)
				data, err := p.Html()
				require.NoError(t, err)
				assert.Equal(t, testCase.expectedRenderedContent, strings.TrimSpace(data))
			})
		}
	})
}
