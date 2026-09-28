// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"net/http"
	"net/url"
	"testing"

	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	"forgejo.org/tests"

	"github.com/stretchr/testify/assert"
)

func TestRenderEndpoint(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user2")

		t.Run("Normal", func(t *testing.T) {
			t.Run("Empty repository", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", "/render/user11/repo9/README.md")
				session.MakeRequest(t, req, http.StatusNotFound)
			})

			t.Run("Markdown", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", "/user2/repo1/render/branch/master/README.md")
				resp := session.MakeRequest(t, req, http.StatusOK)
				const expected = "<h1 id=\"user-content-repo1\" dir=\"auto\">repo1</h1>\n<p dir=\"auto\">Description for repo1</p>\n"
				assert.Equal(t, expected, resp.Body.String())
			})
		})

		t.Run("LFS", func(t *testing.T) {
			t.Run("ServerEnabled", func(t *testing.T) {
				defer test.MockVariableValue(&setting.LFS.StartServer, true)()

				t.Run("Markdown", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", "/user2/lfs/render/branch/master/subdir/README.md")
					resp := session.MakeRequest(t, req, http.StatusOK)
					const expected = "<h1 id=\"user-content-testing-readmes-in-lfs\" dir=\"auto\">Testing READMEs in LFS</h1>\n"
					assert.Equal(t, expected, resp.Body.String())
				})

				t.Run("Image", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", "/user2/lfs/render/branch/master/jpeg.jpg")
					resp := session.MakeRequest(t, req, http.StatusInternalServerError)
					const expected = "Unsupported file type render\n"
					assert.Equal(t, expected, resp.Body.String())
				})
			})

			t.Run("ServerDisabled", func(t *testing.T) {
				defer test.MockVariableValue(&setting.LFS.StartServer, false)()

				t.Run("Markdown", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", "/user2/lfs/render/branch/master/subdir/README.md")
					resp := session.MakeRequest(t, req, http.StatusOK)
					const expected = "<p dir=\"auto\">version <a href=\"https://git-lfs.github.com/spec/v1\" rel=\"nofollow\">https://git-lfs.github.com/spec/v1</a>\noid sha256:9d172e5c64b4f0024b9901ec6afe9ea052f3c9b6ff9f4b07956d8c48c86fca82\nsize 25</p>\n"
					assert.Equal(t, expected, resp.Body.String())
				})

				// TODO: This must be corrected, but should suffice for now.
				// See FIXME in routers/web/repo/view.go.
				t.Run("Image", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", "/user2/lfs/render/branch/master/jpeg.jpg")
					resp := session.MakeRequest(t, req, http.StatusOK)
					const expected = "version https://git-lfs.github.com/spec/v1\noid sha256:0b8d8b5f15046343fd32f451df93acc2bdd9e6373be478b968e4cad6b6647351\nsize 107\n"
					assert.Equal(t, expected, resp.Body.String())
				})
			})
		})
	})
}
