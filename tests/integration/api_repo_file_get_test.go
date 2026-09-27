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
	api "forgejo.org/modules/structs"
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

				// checking for the lfs file in the correct repo with proper token.
				reqLFS := NewRequest(
					t, "GET",
					fmt.Sprintf("/api/v1/repos/%s/%s/media/%s", httpContext.Username, httpContext.Reponame, lfs),
				).AddTokenAuth(httpContext.Token)
				respLFS := MakeRequestNilResponseRecorder(t, reqLFS, http.StatusOK)
				assert.Equal(t, littleSize, respLFS.Length)

				// checking for the lfs file in an incorrect (i.e. another) repo.
				MakeRequestNilResponseRecorder(t, NewRequest(
					t, "GET",
					fmt.Sprintf("/api/v1/repos/%s/%s/media/%s", httpContext.Username, "repo2", lfs),
				).AddTokenAuth(httpContext.Token), http.StatusNotFound)

				// checking for the lfs file in the correct repo, but without token with right scopes.
				MakeRequestNilResponseRecorder(t, NewRequest(
					t, "GET",
					fmt.Sprintf("/api/v1/repos/%s/%s/media/%s", httpContext.Username, httpContext.Reponame, lfs),
				), http.StatusNotFound)

				doAPIDeleteRepository(httpContext)(t)
			})(t)
		})
	})
}
