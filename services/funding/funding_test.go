// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package funding

import (
	"fmt"
	"testing"

	"forgejo.org/models/repo"
	"forgejo.org/modules/setting"
	api "forgejo.org/modules/structs"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultSettings(t *testing.T) func() {
	t.Helper()

	resetFundingProviders := test.MockProtect(&setting.FundingProviders)
	resetMaxFundingEntries := test.MockProtect(&setting.MaxFundingEntriesPerConfig)
	resetValidSiteURLSchemes := test.MockProtect(&setting.Service.ValidSiteURLSchemes)

	// These should align with server defaults:
	setting.LoadBuiltInFundingProviders()
	setting.MaxFundingEntriesPerConfig = 15
	setting.Service.ValidSiteURLSchemes = []string{"http", "https"}

	return func() {
		resetFundingProviders()
		resetMaxFundingEntries()
		resetValidSiteURLSchemes()
	}
}

func getFundingFromConfig(t *testing.T, config string) ([]*api.RepoFundingEntry, []error) {
	funding, errs, err := getFundingFromBlob([]byte(config))
	require.NoError(t, err)
	return funding, errs
}

func assertEntry(t *testing.T, entry *api.RepoFundingEntry, expectedProvider, expectedTitle, expectedValue string) {
	t.Helper()
	assert.Equal(t, expectedProvider, entry.ProviderName)
	assert.Equal(t, expectedTitle, entry.Title)
	assert.Equal(t, expectedValue, entry.Value)
}

func assertCustom(t *testing.T, entry *api.RepoFundingEntry, expectedURL string) {
	t.Helper()
	// different from the others, "custom" entries display their URL value verbatim:
	assertEntry(t, entry, "custom", expectedURL, expectedURL)
}

func assertGithub(t *testing.T, entry *api.RepoFundingEntry, expectedTitle, expectedValue string) {
	t.Helper()
	assertEntry(t, entry, "github", expectedTitle, expectedValue)
}

func assertKoFi(t *testing.T, entry *api.RepoFundingEntry, expectedTitle, expectedValue string) {
	t.Helper()
	assertEntry(t, entry, "ko_fi", expectedTitle, expectedValue)
}

func assertLiberapay(t *testing.T, entry *api.RepoFundingEntry, expectedTitle, expectedValue string) {
	t.Helper()
	assertEntry(t, entry, "liberapay", expectedTitle, expectedValue)
}

func assertPatreon(t *testing.T, entry *api.RepoFundingEntry, expectedTitle, expectedValue string) {
	t.Helper()
	assertEntry(t, entry, "patreon", expectedTitle, expectedValue)
}

func assertTidelift(t *testing.T, entry *api.RepoFundingEntry, expectedTitle, expectedValue string) {
	t.Helper()
	assertEntry(t, entry, "tidelift", expectedTitle, expectedValue)
}

func TestFundingErrorIdentityPrimitives(t *testing.T) {
	cases := [][2]error{
		// {<error case>, <expected type>}
		{ErrFundingNotExist{Repo: &repo.Repository{}}, ErrFundingNotExist{}},
		{ErrUnknownFundingProvider{Name: "SomeName"}, ErrUnknownFundingProvider{}},
		{ErrTooManyFundingProviders{TotalLimit: 50}, ErrTooManyFundingProviders{}},
		{ErrDuplicateFundingEntry{Name: "SomeName", Value: "SomeValue"}, ErrDuplicateFundingEntry{}},
		{ErrBadInput{Name: "SomeName"}, ErrBadInput{}},
		{ErrCannotParseURL{Name: "SomeName", Err: fmt.Errorf("test")}, ErrCannotParseURL{}},
		{ErrBadURLScheme{GivenScheme: "gemini", ValidSchemes: []string{"http", "https"}}, ErrBadURLScheme{}},
		{ErrInvalidYamlType{Name: "SomeName"}, ErrInvalidYamlType{}},
	}
	for _, c := range cases {
		err := c[0]
		kind := c[1]
		require.ErrorIs(t, err, kind) // e.g. errors.Is(err, ErrFundingNotExist{})

		wrappedErr := fmt.Errorf("wrapped: %w", err)
		require.ErrorIs(t, wrappedErr, kind)
	}
}

func TestFundingConfigParseErrors(t *testing.T) {
	configs := []string{
		`this isn't yaml`,
		`[`,
		`]`,
		`foo`,
		`custom: Arbitrary: text`,
		"custom:\n\t- foo",
	}

	for _, config := range configs {
		data, lineErrors, err := getFundingFromBlob([]byte(config))
		require.Error(t, err)
		require.Nil(t, data)
		require.Nil(t, lineErrors)
		// there's a huge variety of possible YAML parse errors, so we won't bother testing that they're exact.
		// what's important here is that we get an error instead of data.
	}
}

func TestFundingEntriesFromConfig(t *testing.T) {
	defer defaultSettings(t)()

	t.Run("Empty config", func(t *testing.T) {
		funding, errs := getFundingFromConfig(t, "")
		assert.Empty(t, errs)
		assert.Empty(t, funding)
	})

	t.Run("Custom string value", func(t *testing.T) {
		configs := [][2]string{
			{`custom: "https://a.com"`, "https://a.com"},
			{`custom: https://a.com`, "https://a.com"},
			{`custom: b.com`, "https://b.com"},
			{`custom: 'http://b.com'`, "http://b.com"},
			{`custom: "http://withquery.example.com?test=foo"`, "http://withquery.example.com?test=foo"},
			{`custom: http://withquery.example.com?test=foo`, "http://withquery.example.com?test=foo"},
			{`custom: "http://thistimewithhash#foo"`, "http://thistimewithhash#foo"},
			{`custom: http://thistimewithhash#foo`, "http://thistimewithhash#foo"},
			{`custom: https://a.com`, "https://a.com"},
			{`custom: http://withquery.example.com?test=foo`, "http://withquery.example.com?test=foo"},
			{`custom: http://thistimewithhash#foo`, "http://thistimewithhash#foo"},
			{`custom: 'localhost:1234'`, "https://localhost:1234"},
			{`custom: localhost:1234`, "https://localhost:1234"},
			{`custom: 'http://localhost:1234'`, "http://localhost:1234"},
			{`custom: "https://localhost:8080/"`, "https://localhost:8080/"},
			{`custom: "https://[::]:8080/"`, "https://[::]:8080/"},
			{`custom: '😀.com'`, "https://xn--e28h.com"},
			// adapted from the list at https://en.wikipedia.org/wiki/International_email:
			{`custom: '例子.广告'`, "https://xn--fsqu00a.xn--4rr70v"},
			{`custom: 'ಡೇಟಾಮೇಲ್.ಭಾರತ'`, "https://xn--xscd4bq2d9bd3c.xn--2scrj9c"},
			{`custom: 'डाटा.भारत'`, "https://xn--c2bd1gb.xn--h2brj9c"},
			{`custom: 'пошта.укр'`, "https://xn--80a1acn3a.xn--j1amh"},
			{`custom: 'παράδειγμα.ελ'`, "https://xn--hxajbheg2az3al.xn--qxam"},
			{`custom: 'Sörensen.example.com'`, "https://xn--srensen-90a.example.com"},
			{`custom: 'пример.рф'`, "https://xn--e1afmkfd.xn--p1ai"},
			{`custom: 'موقع.عر'`, "https://xn--4gbrim.xn--wgbp"},
		}
		for _, configCase := range configs {
			config := configCase[0]
			expectedURL := configCase[1]

			funding, errs := getFundingFromConfig(t, config)
			assert.Empty(t, errs)
			assert.Len(t, funding, 1)
			assertCustom(t, funding[0], expectedURL)
		}
	})

	t.Run("Invalid custom string value", func(t *testing.T) {
		configs := []string{
			`custom: 'localhost:text'`,
			`custom: localhost:text`,
			`custom: 'Arbitrary: text'`,
			`custom: "h3://localhost:8080/"`,
			`custom: ' '`,
			`custom: [[test]]`,
			`custom: [["test"]]`,
			`custom: [[]]`,
			`custom:`,
			`custom: 42`,
			"custom: 42\nwhatever:",
		}
		for _, config := range configs {
			funding, errs := getFundingFromConfig(t, config)
			assert.NotEmpty(t, errs)
			assert.Empty(t, funding)
		}
	})

	t.Run("Custom string array", func(t *testing.T) {
		config := "custom:\n" +
			`- "https://a.com"` + "\n" +
			"- b.com\n" +
			`- "http://withquery.example.com?test=foo"` + "\n" +
			`- "http://thistimewithhash#foo"` + "\n"
		funding, errs := getFundingFromConfig(t, config)
		assert.Empty(t, errs)

		require.Len(t, funding, 4)
		assertCustom(t, funding[0], "https://a.com")
		assertCustom(t, funding[1], "https://b.com")
		assertCustom(t, funding[2], "http://withquery.example.com?test=foo")
		assertCustom(t, funding[3], "http://thistimewithhash#foo")
	})
}

func TestFundingEntriesWithErrorsFromConfig(t *testing.T) {
	defer defaultSettings(t)()

	t.Run("Skips duplicate entries", func(t *testing.T) {
		configs := [][3]any{
			{`custom: ["https://a.com", "https://a.com", "https://b.com"]`, "https://a.com", []string{"https://a.com", "https://b.com"}},
			{`custom: [test1, test1, test2]`, "https://test1", []string{"https://test1", "https://test2"}},
			{`custom: [test1, "https://test1", test2]`, "https://test1", []string{"https://test1", "https://test2"}},
		}
		for _, configCase := range configs {
			config := configCase[0].(string)
			expectedDuplicate := configCase[1].(string)
			expectedRemainder := configCase[2].([]string)
			funding, errs := getFundingFromConfig(t, config)

			assert.Len(t, errs, 1)
			assert.Equal(t, fmt.Sprintf("Duplicate entry for key 'custom': %s", expectedDuplicate), errs[0].Error())

			actualRemainder := make([]string, 0, len(funding))
			for _, funding := range funding {
				actualRemainder = append(actualRemainder, funding.Value)
			}
			assert.Equal(t, expectedRemainder, actualRemainder)
		}
	})

	t.Run("Partially invalid (bad key omitted)", func(t *testing.T) {
		config := "liberapay: test\n" +
			"ko_fi:\n" +
			`custom: [test, "https://example.com", 'Arbitrary:4242', 'Arbitrary: 4242', 'h3://localhost']`
		funding, errs := getFundingFromConfig(t, config)

		assert.Len(t, errs, 3)
		assert.Equal(t, "Invalid type for key 'ko_fi', expected a string or string array", errs[0].Error())
		assert.Equal(t, `Invalid URL value for key 'custom': parse "https://Arbitrary: 4242": invalid port ": 4242" after host`, errs[1].Error())
		assert.Equal(t, `Invalid URL value for key 'custom': invalid scheme "h3", expected one of: http, https`, errs[2].Error())

		assert.Len(t, funding, 4)
		assertLiberapay(t, funding[0], "liberapay.com/test", "https://liberapay.com/test")
		assertCustom(t, funding[1], "https://test")
		assertCustom(t, funding[2], "https://example.com")
		assertCustom(t, funding[3], "https://arbitrary:4242")
	})

	t.Run("Partially invalid (unknown key omitted)", func(t *testing.T) {
		configs := []string{
			"whatever: test\ncustom: [test, \"https://example.com\"]",
			"whatever: 42\ncustom: [test, \"https://example.com\"]", // bad key type has same error
		}

		for _, config := range configs {
			funding, errs := getFundingFromConfig(t, config)

			assert.Len(t, errs, 1)
			assert.Equal(t, "Unknown funding provider: whatever", errs[0].Error())

			assert.Len(t, funding, 2)
			assertCustom(t, funding[0], "https://test")
			assertCustom(t, funding[1], "https://example.com")
		}
	})

	t.Run("Partially invalid (bad known keys omitted)", func(t *testing.T) {
		config := "whatever: test\n" +
			`ko_fi: [42, test, "https://example.com"]`
		funding, errs := getFundingFromConfig(t, config)

		assert.Len(t, errs, 3)
		assert.Equal(t, "Unknown funding provider: whatever", errs[0].Error())
		assert.Equal(t, "Invalid type for key 'ko_fi', expected a string or string array", errs[1].Error())
		assert.Equal(t, "Value for key 'ko_fi' does not match pattern /^[^/]+$/", errs[2].Error())

		assert.Len(t, funding, 1)
		assertKoFi(t, funding[0], "ko-fi.com/test", "https://ko-fi.com/test")
	})

	t.Run("Partially invalid (one element of list is bad type)", func(t *testing.T) {
		config := `patreon: [42, "example"]`
		funding, errs := getFundingFromConfig(t, config)

		assert.Len(t, errs, 1)
		assert.Equal(t, "Invalid type for key 'patreon', expected a string or string array", errs[0].Error())

		assert.Len(t, funding, 1)
		assertPatreon(t, funding[0], "patreon.com/example", "https://patreon.com/example")
	})

	t.Run("Partially invalid (too many of one provider)", func(t *testing.T) {
		config := "custom:\n" +
			"- test1\n" +
			`- "https://example.com"` + "\n" +
			"- test3\n" +
			"- test4\n" +
			"- test5\n" +
			"- test6\n" +
			"- test7\n" +
			"- test8\n" +
			"- test9\n" +
			"- test10\n" +
			"- test11\n" +
			"- test12\n" +
			"- test13\n" +
			"- test14\n" +
			"- test15\n" +
			"- too_many"
		funding, errs := getFundingFromConfig(t, config)

		assert.Len(t, errs, 1)
		assert.Equal(t, "Expected up to 15 funding providers", errs[0].Error())

		assert.Len(t, funding, 15)
		assertCustom(t, funding[0], "https://test1")
		assertCustom(t, funding[1], "https://example.com")
		assertCustom(t, funding[2], "https://test3")
		assertCustom(t, funding[3], "https://test4")
		assertCustom(t, funding[4], "https://test5")
		assertCustom(t, funding[5], "https://test6")
		assertCustom(t, funding[6], "https://test7")
		assertCustom(t, funding[7], "https://test8")
		assertCustom(t, funding[8], "https://test9")
		assertCustom(t, funding[9], "https://test10")
		assertCustom(t, funding[10], "https://test11")
		assertCustom(t, funding[11], "https://test12")
		assertCustom(t, funding[12], "https://test13")
		assertCustom(t, funding[13], "https://test14")
		assertCustom(t, funding[14], "https://test15")
	})

	t.Run("Partially invalid (too many of two providers)", func(t *testing.T) {
		config := "ko_fi: [test]\n" +
			"tidelift: npm/example\n" +
			"custom:\n" +
			"- test1\n" +
			`- "https://example.com"` + "\n" +
			"- test3\n" +
			"- test4\n" +
			"- test5\n" +
			"- test6\n" +
			"- test7\n" +
			"- test8\n" +
			"- test9\n" +
			"- test10\n" +
			"- test11\n" +
			"- test12\n" +
			"- test13\n" +
			"- too_many"
		funding, errs := getFundingFromConfig(t, config)

		assert.Len(t, errs, 1)
		assert.Equal(t, "Expected up to 15 funding providers", errs[0].Error())

		assert.Len(t, funding, 15)
		assertKoFi(t, funding[0], "ko-fi.com/test", "https://ko-fi.com/test")
		assertTidelift(t, funding[1], "tidelift.com/funding/github/npm/example", "https://tidelift.com/funding/github/npm/example")
		assertCustom(t, funding[2], "https://test1")
		assertCustom(t, funding[3], "https://example.com")
		assertCustom(t, funding[4], "https://test3")
		assertCustom(t, funding[5], "https://test4")
		assertCustom(t, funding[6], "https://test5")
		assertCustom(t, funding[7], "https://test6")
		assertCustom(t, funding[8], "https://test7")
		assertCustom(t, funding[9], "https://test8")
		assertCustom(t, funding[10], "https://test9")
		assertCustom(t, funding[11], "https://test10")
		assertCustom(t, funding[12], "https://test11")
		assertCustom(t, funding[13], "https://test12")
		assertCustom(t, funding[14], "https://test13")
	})

	t.Run("Partially invalid (too many of two providers with more following)", func(t *testing.T) {
		config := "ko_fi: [test]\n" +
			"tidelift: npm/example\n" +
			"custom:\n" +
			"- test1\n" +
			`- "https://example.com"` + "\n" +
			"- test3\n" +
			"- test4\n" +
			"- test5\n" +
			"- test6\n" +
			"- test7\n" +
			"- test8\n" +
			"- test9\n" +
			"- test10\n" +
			"- test11\n" +
			"- test12\n" +
			"- test13\n" +
			"- too_many\n" +
			"liberapay: still_too_many"
		funding, errs := getFundingFromConfig(t, config)

		assert.Len(t, errs, 1)
		assert.Equal(t, "Expected up to 15 funding providers", errs[0].Error())

		assert.Len(t, funding, 15)
		assertKoFi(t, funding[0], "ko-fi.com/test", "https://ko-fi.com/test")
		assertTidelift(t, funding[1], "tidelift.com/funding/github/npm/example", "https://tidelift.com/funding/github/npm/example")
		assertCustom(t, funding[2], "https://test1")
		assertCustom(t, funding[3], "https://example.com")
		assertCustom(t, funding[4], "https://test3")
		assertCustom(t, funding[5], "https://test4")
		assertCustom(t, funding[6], "https://test5")
		assertCustom(t, funding[7], "https://test6")
		assertCustom(t, funding[8], "https://test7")
		assertCustom(t, funding[9], "https://test8")
		assertCustom(t, funding[10], "https://test9")
		assertCustom(t, funding[11], "https://test10")
		assertCustom(t, funding[12], "https://test11")
		assertCustom(t, funding[13], "https://test12")
		assertCustom(t, funding[14], "https://test13")
	})

	t.Run("Partially invalid (too many of all providers)", func(t *testing.T) {
		defer test.MockProtect(&setting.MaxFundingEntriesPerConfig)()
		setting.MaxFundingEntriesPerConfig = 5 // lower min so we can test that fail case

		config := "ko_fi: [test]\n" +
			"tidelift: npm/example\n" +
			"patreon: example\n" +
			"liberapay: example\n" +
			"github: example\n" +
			"issuehunt: too_many\n" +
			"open_collective: still_too_many\n"
		funding, errs := getFundingFromConfig(t, config)

		assert.Len(t, errs, 1)
		assert.Equal(t, "Expected up to 5 funding providers", errs[0].Error()) // error message reflects config

		assert.Len(t, funding, 5)
		assertKoFi(t, funding[0], "ko-fi.com/test", "https://ko-fi.com/test")
		assertTidelift(t, funding[1], "tidelift.com/funding/github/npm/example", "https://tidelift.com/funding/github/npm/example")
		assertPatreon(t, funding[2], "patreon.com/example", "https://patreon.com/example")
		assertLiberapay(t, funding[3], "liberapay.com/example", "https://liberapay.com/example")
		assertGithub(t, funding[4], "github.com/sponsors/example", "https://github.com/sponsors/example")
	})
}

func TestFundingEntriesWithCustomSchemes(t *testing.T) {
	defer defaultSettings(t)()

	t.Run("an HTTPS website under default schemes", func(t *testing.T) {
		config := "custom: 'https://example.com'"
		funding, errs := getFundingFromConfig(t, config)

		assert.Empty(t, errs)
		assert.Len(t, funding, 1)
		assertCustom(t, funding[0], "https://example.com")
	})

	t.Run("an H3 website under default schemes", func(t *testing.T) {
		config := "custom: 'h3://example.com'"
		funding, errs := getFundingFromConfig(t, config)

		assert.Empty(t, funding)
		assert.Len(t, errs, 1)
		assert.Equal(t, `Invalid URL value for key 'custom': invalid scheme "h3", expected one of: http, https`, errs[0].Error())
	})

	t.Run("an H3 website under custom schemes", func(t *testing.T) {
		defer test.MockProtect(&setting.Service.ValidSiteURLSchemes)()
		setting.Service.ValidSiteURLSchemes = append(setting.Service.ValidSiteURLSchemes, "h3")

		config := "custom: 'h3://example.com'"
		funding, errs := getFundingFromConfig(t, config)

		assert.Empty(t, errs)
		assert.Len(t, funding, 1)
		assertCustom(t, funding[0], "h3://example.com")
	})

	t.Run("a Gemini website under custom schemes", func(t *testing.T) {
		defer test.MockProtect(&setting.Service.ValidSiteURLSchemes)()
		setting.Service.ValidSiteURLSchemes = append(setting.Service.ValidSiteURLSchemes, "h3")

		config := "custom: 'gemini://example.com'"
		funding, errs := getFundingFromConfig(t, config)

		assert.Empty(t, funding)
		assert.Len(t, errs, 1)
		assert.Equal(t, `Invalid URL value for key 'custom': invalid scheme "gemini", expected one of: http, https, h3`, errs[0].Error())
	})
}
