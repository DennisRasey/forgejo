// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "forgejo.org/models/auth"
	"forgejo.org/modules/git"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIGetRawFileOrLFS(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		t.Run("Empty repository (normal path)", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// /raw/ -> Normal path
			req := NewRequest(t, "GET", "/api/v1/repos/user11/repo9/raw/README.md")
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("Empty repository (LFS path)", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// /media/ -> LFS path
			req := NewRequest(t, "GET", "/api/v1/repos/user11/repo9/media/README.md")
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("Normal raw file", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/media/README.md")
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, "# repo1\n\nDescription for repo1", resp.Body.String())
		})

		t.Run("LFS raw file", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			httpContext := NewAPITestContext(
				t, "user2", "repo-lfs-test",
				auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser,
			)
			doAPICreateRepository(httpContext, nil, git.Sha1ObjectFormat, func(t *testing.T, repository api.Repository) { // FIXME: use forEachObjectFormat
				u.Path = httpContext.GitPath()
				u.User = url.UserPassword("user2", userPassword)

				dstPath := t.TempDir()
				t.Run("Clone", doGitClone(dstPath, u))

				dstPath2 := t.TempDir()
				t.Run("Partial Clone", doPartialGitClone(dstPath2, u))

				lfs, _ := lfsCommitAndPushTest(t, dstPath)

				// checking for the lfs file in an incorrect (i.e. another) repo.
				t.Run("WrongRepo", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					MakeRequestNilResponseRecorder(t, NewRequest(
						t, "GET",
						fmt.Sprintf("/api/v1/repos/%s/%s/media/%s", httpContext.Username, "repo2", lfs),
					).AddTokenAuth(httpContext.Token), http.StatusNotFound)
				})

				// checking for the lfs file in the correct repo, but without token with right scopes.
				t.Run("RightRepoInappropriateToken", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					MakeRequestNilResponseRecorder(t, NewRequest(
						t, "GET",
						fmt.Sprintf("/api/v1/repos/%s/%s/media/%s", httpContext.Username, httpContext.Reponame, lfs),
					), http.StatusNotFound)
				})

				// checking for the lfs file in the correct repo with proper token.
				t.Run("ServeIndirect", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()
					defer test.MockVariableValue(&setting.LFS.Storage.MinioConfig.ServeDirect, false)()

					reqLFS := NewRequest(
						t, "GET",
						fmt.Sprintf("/api/v1/repos/%s/%s/media/%s", httpContext.Username, httpContext.Reponame, lfs),
					).AddTokenAuth(httpContext.Token)
					respLFS := MakeRequest(t, reqLFS, http.StatusOK)
					assert.Equal(t, "application/octet-stream", respLFS.Header().Get("Content-Type"))

					assert.Contains(t, respLFS.Header().Get("Content-Disposition"), "inline; filename=\"lfs-data-file-")
					assert.Contains(t, respLFS.Header().Get("Content-Disposition"), "filename*=UTF-8''lfs-data-file-")

					assert.Equal(t, "1024", respLFS.Header().Get("Content-Length"))
					assert.Empty(t, test.RedirectURL(respLFS))

					assert.NotEmpty(t, respLFS.Body.String())
					// Sanity check, caching tested in modules/httpcache/httpcache_test.go
					//
					// This is here to check that the redirect (URL -> file) is OK.
					assert.Equal(t, "file", respLFS.Header().Get("X-Forgejo-Object-Type"))
				})

				// checking for the lfs file in the correct repo with proper token.
				t.Run("ServeDirect", func(t *testing.T) {
					if setting.LFS.Storage.Type != setting.MinioStorageType {
						t.Skip("Test skipped for non-Minio-storage.")
						return
					}

					defer tests.PrintCurrentTest(t)()
					defer test.MockVariableValue(&setting.LFS.Storage.MinioConfig.ServeDirect, true)()

					reqLFS := NewRequest(
						t, "GET",
						fmt.Sprintf("/api/v1/repos/%s/%s/media/%s", httpContext.Username, httpContext.Reponame, lfs),
					).AddTokenAuth(httpContext.Token)
					respLFS := MakeRequest(t, reqLFS, http.StatusSeeOther)
					assert.Equal(t, "text/html; charset=utf-8", respLFS.Header().Get("Content-Type"))

					assert.Empty(t, respLFS.Header().Get("Content-Disposition"))
					assert.Empty(t, respLFS.Header().Get("Content-Length"))
					assert.NotEmpty(t, respLFS.Header().Get("Location"))

					assert.NotEmpty(t, test.RedirectURL(respLFS))
					assert.Contains(t, test.RedirectURL(respLFS), "&response-content-disposition=attachment%3B%20filename%3D%22lfs-data-file-")

					assert.NotEmpty(t, respLFS.Body.String())
					// Sanity check, caching tested in modules/httpcache/httpcache_test.go
					//
					// This is here to check that the redirect (URL -> file) is OK.
					assert.Equal(t, "file", respLFS.Header().Get("X-Forgejo-Object-Type"))
				})

				doAPIDeleteRepository(httpContext)(t)
			})(t)
		})
	})
}
