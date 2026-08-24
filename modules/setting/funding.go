// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package setting

import (
	"cmp"
	"regexp"
	"strings"

	"forgejo.org/modules/log"
)

// A funding provider, as it appears in the server config
type FundingProviderConfig struct {
	// The name of the funding platform
	Name string

	// A template string that defines a URL, ideally to a given user profile, to
	// which users should be sent to support a project. This string should
	// contain at least one instance of '%s' or '%[1]s', which Forgejo will
	// replace with the string given in a repo's funding config.
	//
	// This is the only required config key; the other details may be derived
	// from this and the platform name.
	Template string

	// A regular expression which input values must match before they may be
	// interpolated into the `Template`.
	//
	// The default value permits a single path segment.
	InputPattern *regexp.Regexp

	// A template string that defines the text that should show in place of a URL
	// in the UI. This string should contain at least one instance of %s or
	// %[1]s, which will be replaced with the string given in a repo's funding
	// config.
	//
	// When parsed from the server config, this value defaults to the value of
	// `Template`, without the URI scheme if present.
	Title string
}

var FundingProviders map[string]*FundingProviderConfig

// The maximum number of funding entries that may be present in a given funding
// config, regardless of how many of those entries share a funding provider.
var MaxFundingEntriesPerConfig int

// Matches any formatting sigil, e.g. '%s', '%d', '%[3]f', '%%', etc.
const sigilPattern = `%(\[\d+\])?.`

// Normalizes the given string for use as a formatting string that takes a
// single input.
//
// Returns the given string, with the given modifications:
//   - all instances of '%s' are transformed into '%[1]s'
//   - all other formatting sigils (%d, %[2]s, etc.) are escaped by prepending
//     '%', e.g. '%%d'
//   - any dangling '%' characters are transformed into '%%'
//
// Note that the resulting string may not contain a valid '%[1]s' sigil. Care
// should be taken to ensure that the string can be used as a template string
// before attempting to use it with fmt.Sprintf, to avoid unexpected errors.
func cleanUpSigils(s string) string {
	if s == "%" {
		return "%%" // escape away our lone percent sign
	} else if len(s) < 2 {
		return s // input too short to contain other formatting sigils
	}

	if strings.HasSuffix(s, "%") && !strings.HasSuffix(s, "%%") {
		s += "%" // escape away the trailing percent sign
	}

	sigilRegex := regexp.MustCompile(sigilPattern)
	return sigilRegex.ReplaceAllStringFunc(s, func(match string) string {
		if match == "%%" || match == "%[1]s" {
			return match // already safe
		} else if !strings.HasSuffix(match, "s") || strings.HasPrefix(match, "%[") {
			return "%" + match // escape away non-string or index-other-than-1 sigils
		}
		return "%[1]s" // positional string sigils become explicitly index-1
	})
}

// Returns a value limited to the given inclusive range, and `true` if the
// given value fell outside of that range.
//
// If T is a floating-point type and any of the arguments are NaNs, clamp will return NaN.
func clamp[T cmp.Ordered](n, minN, maxN T) (new T, didClamp bool) {
	new = min(max(n, minN), maxN)
	return new, new != n
}

func addFundingProvider(provider *FundingProviderConfig) {
	FundingProviders[provider.Name] = provider
}

// GetFundingProviderByName returns a reference to the configured funding
// provider that has the given Name, or nil if Forgejo knows no such provider.
func GetFundingProviderByName(name string) *FundingProviderConfig {
	return FundingProviders[name]
}

const (
	singleSegmentPattern = `^[^/]+$`               // e.g. "example"
	twoSegmentPattern    = `^[^/]+\/[^/]+$`        // e.g. "a/example"
	threeSegmentPattern  = `^[^/]+\/[^/]+\/[^/]+$` // e.g. "a/b/c"
	anythingPattern      = `.+`                    // e.g. "http://a.com"
)

func LoadBuiltInFundingProviders() {
	FundingProviders = make(map[string]*FundingProviderConfig)

	singleSegmentRegex := regexp.MustCompile(singleSegmentPattern)
	twoSegmentRegex := regexp.MustCompile(twoSegmentPattern)
	threeSegmentRegex := regexp.MustCompile(threeSegmentPattern)
	anythingRegex := regexp.MustCompile(anythingPattern)

	// built-in providers are largely based on github's list at <https://docs.github.com/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/displaying-a-sponsor-button-in-your-repository#about-funding-files>
	// `polar` is intentionally omitted, see https://codeberg.org/forgejo/forgejo/pulls/13361#issuecomment-20078290
	addFundingProvider(&FundingProviderConfig{
		Name:         "community_bridge", // aka LFX Mentorship, but the config calls it community_bridge for compat
		Title:        "crowdfunding.linuxfoundation.org/initiatives/%[1]s",
		Template:     "https://crowdfunding.linuxfoundation.org/initiatives/%[1]s", // originally https://funding.communitybridge.org/projects/*
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "github",
		Title:        "github.com/sponsors/%[1]s",
		Template:     "https://github.com/sponsors/%[1]s",
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "issuehunt",
		Title:        "issuehunt.io/r/%[1]s",
		Template:     "https://issuehunt.io/r/%[1]s",
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "ko_fi",
		Title:        "ko-fi.com/%[1]s",
		Template:     "https://ko-fi.com/%[1]s",
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "liberapay",
		Title:        "liberapay.com/%[1]s",
		Template:     "https://liberapay.com/%[1]s",
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "open_collective",
		Title:        "opencollective.com/%[1]s",
		Template:     "https://opencollective.com/%[1]s",
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "patreon",
		Title:        "patreon.com/%[1]s",
		Template:     "https://patreon.com/%[1]s",
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "tidelift",
		Title:        "tidelift.com/funding/github/%[1]s",
		Template:     "https://tidelift.com/funding/github/%[1]s",
		InputPattern: twoSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "buy_me_a_coffee",
		Title:        "buymeacoffee.com/%[1]s",
		Template:     "https://buymeacoffee.com/%[1]s",
		InputPattern: singleSegmentRegex,
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "thanks_dev",
		Title:        "thanks.dev/%[1]s",
		Template:     "https://thanks.dev/%[1]s",
		InputPattern: threeSegmentRegex, // we expect something like "u/gh/example"
	})
	addFundingProvider(&FundingProviderConfig{
		Name:         "custom",
		Title:        "%[1]s",
		Template:     "%[1]s",
		InputPattern: anythingRegex, // matches anything; the final value is treated like a URL in any case
	})
}

func loadCustomFundingProvidersFrom(rootCfg ConfigProvider) {
	LoadBuiltInFundingProviders()

	const keyMaxFundingEntriesPerConfig = "MAX_FUNDING_ENTRIES_PER_CONFIG"
	const keyTitle = "TITLE"
	const keyTemplate = "TEMPLATE"
	const keyInputPattern = "INPUT_PATTERN"

	fundingSection := rootCfg.Section("funding")

	MaxFundingEntriesPerConfig = fundingSection.Key(keyMaxFundingEntriesPerConfig).MustInt(15)
	newLimit, didClamp := clamp(MaxFundingEntriesPerConfig, 0, 20) // arbitrary "reasonable" max
	if didClamp {
		log.Warn("%s.%s name is out of bounds, clamping to %d", fundingSection.Name(), keyMaxFundingEntriesPerConfig, MaxFundingEntriesPerConfig)
	}
	MaxFundingEntriesPerConfig = newLimit

	for _, sec := range fundingSection.ChildSections() {
		name := strings.TrimPrefix(sec.Name(), "funding.")
		if name == "" {
			log.Warn("name is empty, funding %s ignored", sec.Name())
			continue
		}

		rawTitle := sec.Key(keyTitle).MustString("")
		rawTemplate := sec.Key(keyTemplate).MustString("")
		rawInputPattern := sec.Key(keyInputPattern).MustString(singleSegmentPattern)

		inputPattern, err := regexp.Compile(rawInputPattern)
		if err != nil {
			singleSegmentRegex := regexp.MustCompile(singleSegmentPattern)
			log.Warn("%s.%s %v, using /%s/ instead", sec.Name(), keyInputPattern, err, singleSegmentRegex.String())
			inputPattern = singleSegmentRegex
		}

		template := cleanUpSigils(rawTemplate)
		if !strings.Contains(template, "%[1]s") {
			log.Warn("%s.%s contains no valid instances of '%%[1]s' or '%%s', funding %[1]s ignored", sec.Name(), keyTemplate)
			continue
		}

		// get the url scheme, if any
		scheme, _, found := strings.Cut(template, "://") // e.g. "https://localhost/%s"
		if !found {
			scheme = ""
		}

		// default title to only template minus scheme
		title := rawTitle
		if title == "" {
			title = strings.TrimPrefix(template, scheme+"://")
			// the sigils are already tidy here, no need to clean them up again!
		} else {
			title = cleanUpSigils(title)
		}

		provider := new(FundingProviderConfig)
		provider.Name = name
		provider.Title = title
		provider.Template = template
		provider.InputPattern = inputPattern

		if GetFundingProviderByName(name) == nil {
			addFundingProvider(provider)
		} else {
			log.Warn("%s constructs a funding provider that already exists, existing provider %s is unchanged", sec.Name(), name)
		}
	}
}
