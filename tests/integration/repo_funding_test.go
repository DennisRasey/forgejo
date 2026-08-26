// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"forgejo.org/tests"
	"forgejo.org/tests/forgery"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRepoFundingModalLinksToFileViewOnError(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		t.Run("no errors", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
				Files: forgery.MapFS{
					"FUNDING.yml": forgery.MapFile("custom: localhost\n"),
				},
			})

			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s", repo.OwnerName, repo.Name))
			resp := MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			doc.AssertElement(t, ".ui.error.message", false)
		})

		t.Run("has errors", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			config := "custom: localhost\nko_fi: 1337\nko-fi: example\n"
			repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
				Files: forgery.MapFS{
					"FUNDING.yml": forgery.MapFile(config),
				},
			})

			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s", repo.OwnerName, repo.Name))
			resp := MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			doc.AssertElementPredicate(t, ".ui.error.message", func(els *goquery.Selection) {
				require.Equal(t, 1, els.Length())
				assert.Equal(t, "The funding config contains errors.", strings.TrimSpace(els.Text()))
			})
			doc.AssertElement(t, ".ui.error.message a", true)
			doc.AssertAttrEqual(t, ".ui.error.message a", "href", fmt.Sprintf("/%s/%s/src/branch/main/FUNDING.yml", repo.OwnerName, repo.Name))
		})
	})
}

func TestRepoFundingErrorReadoutOnFileView(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		errors := [][4]any{
			{
				"duplicate YAML key",
				"custom: localhost\ncustom: 'but what about second custom??'\n",
				"Error parsing funding config: Duplicate YAML key: custom",
				[]string{},
			},
			{
				"unexpected root type",
				"not_the_mapping_youre_looking_for\n",
				"Error parsing funding config: Expected YAML mapping, got str",
				[]string{},
			},
			{
				"unexpected value type",
				"custom:\n- localhost\n- [localhost]\n",
				"Error parsing funding config: Invalid type for key \"custom\", expected a string or string array",
				[]string{},
			},
			{
				"multiple errors",
				"ko_fi: 1337\nko-fi: example\n",
				"Errors parsing funding config:",
				[]string{
					"Invalid type for key \"ko_fi\", expected a string or string array",
					"Unknown funding provider: ko-fi",
				},
			},
		}

		sel := ".non-diff-file-content > .ui.error.message"
		user := forgery.CreateUser(t, nil)

		t.Run("no error", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			repo := forgery.CreateRepository(t, user, &forgery.CreateRepositoryOptions{
				Files: forgery.MapFS{
					"FUNDING.yml": forgery.MapFile("custom: localhost\n"),
				},
			})

			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/src/branch/main/FUNDING.yml", repo.OwnerName, repo.Name))
			resp := MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			doc.AssertElement(t, sel, false)
		})

		for _, c := range errors {
			name := c[0].(string)
			config := c[1].(string)
			expectedErr := c[2].(string)
			expectedSubErrs := c[3].([]string)

			repo := forgery.CreateRepository(t, user, &forgery.CreateRepositoryOptions{
				Files: forgery.MapFS{
					"FUNDING.yml": forgery.MapFile(config),
				},
			})

			t.Run(name, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/src/branch/main/FUNDING.yml", repo.OwnerName, repo.Name))
				resp := MakeRequest(t, req, http.StatusOK)
				doc := NewHTMLParser(t, resp.Body)

				// error text should show a translatable error message, not "Unknown error: {whatever}"
				doc.AssertElementPredicate(t, sel, func(els *goquery.Selection) {
					require.Equal(t, 1, els.Length())
					s := strings.TrimSpace(els.Text())
					assert.True(t, strings.HasPrefix(s, expectedErr), "expected prefix: "+expectedErr, "actual: "+s)
					assert.NotContains(t, els.Text(), "Unknown error")
				})

				// multiple errors are read out in a list
				doc.AssertElement(t, sel+" ul", len(expectedSubErrs) != 0)
				doc.AssertElementPredicate(t, sel+" li", func(els *goquery.Selection) {
					require.Equal(t, len(expectedSubErrs), els.Length())
				})
				for idx, expectedErr := range expectedSubErrs {
					// each error text should show a translatable error message, not "Unknown error: {whatever}"
					nth := strconv.Itoa(idx + 1)
					doc.AssertElementPredicate(t, sel+" li:nth-child("+nth+")", func(els *goquery.Selection) {
						assert.Equal(t, expectedErr, strings.TrimSpace(els.Text()))
						assert.NotContains(t, els.Text(), "Unknown error")
					})
				}
			})
		}
	})
}

func TestRepoFundingMitigatesXSS(t *testing.T) {
	onApplicationRun(t, func(t *testing.T, u *url.URL) {
		config := `ko_fi: '"><script>alert(1);</script><a class="'
liberapay: "text/other"
thanks_dev: "could/be/real/bad"
custom:
  - '#" style="background: url(localhost)'
  - 'https://example.com" class="rogue injection'
  - 'https://example.com/" class="rogue injection'
  - '<script>alert` + "`" + "1" + "`" + `</script>'
  - 'Arbitrary: text'
`

		repo := forgery.CreateRepository(t, nil, &forgery.CreateRepositoryOptions{
			Files: forgery.MapFS{
				"FUNDING.yml": forgery.MapFile(config),
			},
		})

		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s", repo.OwnerName, repo.Name))
		resp := MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		doc.AssertElement(t, "#funding-modal", true)
		doc.AssertElement(t, "#funding-modal .ui.error.message", true)
		doc.AssertElementPredicate(t, "#funding-modal li", func(els *goquery.Selection) {
			require.Equal(t, 2, els.Length())
		})

		// list items should contain encoded strings as given in config; these strings should be interpreted as text, NOT as HTML markup
		// strings that don't match the expected format are omitted with error
		doc.AssertAttrEqual(t, "#funding-modal li:nth-child(1) a", "href", "https:#%22%20style=%22background:%20url%28localhost%29")
		doc.AssertElementPredicate(t, "#funding-modal li:nth-child(1) a", func(els *goquery.Selection) {
			assert.Equal(t, "https:#%22%20style=%22background:%20url(localhost)", strings.TrimSpace(els.Text()))
			assert.Equal(t, 0, els.Children().Length())
		})

		doc.AssertAttrEqual(t, "#funding-modal li:nth-child(2) a", "href", "https://example.com/%22%20class=%22rogue%20injection")
		doc.AssertElementPredicate(t, "#funding-modal li:nth-child(2) a", func(els *goquery.Selection) {
			assert.Equal(t, "https://example.com/%22%20class=%22rogue%20injection", strings.TrimSpace(els.Text()))
			assert.Equal(t, 0, els.Children().Length())
		})
	})
}
