// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/stretchr/testify/assert"
)

func TestRepoFundingConfigPrecedence(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		user := forgery.CreateUser(t, nil)
		checkFunding := func(t *testing.T, fundingConfigFilename, configKind string) {
			t.Run("prefers "+configKind+" .forgejo/FUNDING.yml over any "+fundingConfigFilename, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				// Prepare the test repository
				preferredConfig := "custom: example.com\n"
				if configKind == "invalid" {
					preferredConfig = "custom: 42\n"
				}
				altConfig := "ko_fi: test\n"
				repo := forgery.CreateRepository(t, user, &forgery.CreateRepositoryOptions{
					Files: forgery.MapFS{
						".forgejo/FUNDING.yml": forgery.MapFile(preferredConfig),
						fundingConfigFilename:  forgery.MapFile(altConfig),
					},
				})

				// Perform the test
				req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s", repo.OwnerName, repo.Name))
				resp := MakeRequest(t, req, http.StatusOK)
				doc := NewHTMLParser(t, resp.Body)

				// expecting preferredConfig, never altConfig
				fundingEntryCount := doc.Find("#funding-modal li").Length()
				if configKind == "invalid" {
					assert.Zero(t, fundingEntryCount)
				} else {
					assert.Equal(t, 1, fundingEntryCount)
					doc.AssertAttrEqual(t, "#funding-modal li:nth-child(1) a", "href", "https://example.com")
					doc.AssertElement(t, "#funding-modal li:nth-child(1) svg.octicon-link", true)
				}
			})
		}

		checkFunding(t, "FUNDING.yml", "valid")
		checkFunding(t, "FUNDING.yml", "invalid")
		checkFunding(t, ".github/FUNDING.yml", "valid")
		checkFunding(t, ".github/FUNDING.yml", "invalid")
		checkFunding(t, "funding.yml", "valid")
		checkFunding(t, "funding.yml", "invalid")
	})
}
