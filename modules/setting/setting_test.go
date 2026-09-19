// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2025 The Forgejo Authors. All rights reserved
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"forgejo.org/modules/json"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeAbsoluteAssetURL(t *testing.T) {
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234", "https://localhost:2345"))
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234/", "https://localhost:2345"))
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234/", "https://localhost:2345/"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/", "/foo/"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/foo/"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo", "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/bar/"))
}

func TestMakeManifestData(t *testing.T) {
	jsonBytes, err := GetManifestJSON()
	require.NoError(t, err)
	assert.True(t, json.Valid(jsonBytes))
}

func TestMakeManifestDataStandalone(t *testing.T) {
	defer test.MockVariableValue(&PWA.Standalone, true)()

	jsonBytes, err := GetManifestJSON()
	require.NoError(t, err)
	assert.True(t, json.Valid(jsonBytes))
	assert.Contains(t, string(jsonBytes), `"standalone"`)
}

func TestLoadServiceDomainListsForFederation(t *testing.T) {
	oldAppURL := AppURL
	oldFederation := Federation
	oldService := Service

	defer func() {
		AppURL = oldAppURL
		Federation = oldFederation
		Service = oldService
	}()

	cfg, err := NewConfigProviderFromData(`
[federation]
ENABLED = true
[service]
EMAIL_DOMAIN_ALLOWLIST = *.allow.random
EMAIL_DOMAIN_BLOCKLIST = *.block.random
`)

	require.NoError(t, err)
	loadServerFrom(cfg)
	loadFederationFrom(cfg)
	loadServiceFrom(cfg)

	assert.True(t, match(Service.EmailDomainAllowList, "d1.allow.random"))
	assert.True(t, match(Service.EmailDomainAllowList, "localhost"))
}

func TestLoadServiceDomainListsNoFederation(t *testing.T) {
	oldAppURL := AppURL
	oldFederation := Federation
	oldService := Service

	defer func() {
		AppURL = oldAppURL
		Federation = oldFederation
		Service = oldService
	}()

	cfg, err := NewConfigProviderFromData(`
[federation]
ENABLED = false
[service]
EMAIL_DOMAIN_ALLOWLIST = *.allow.random
EMAIL_DOMAIN_BLOCKLIST = *.block.random
`)

	require.NoError(t, err)
	loadServerFrom(cfg)
	loadFederationFrom(cfg)
	loadServiceFrom(cfg)

	assert.True(t, match(Service.EmailDomainAllowList, "d1.allow.random"))
}

func TestLoadServiceDomainListsFederationEmptyAllowList(t *testing.T) {
	oldAppURL := AppURL
	oldFederation := Federation
	oldService := Service

	defer func() {
		AppURL = oldAppURL
		Federation = oldFederation
		Service = oldService
	}()

	cfg, err := NewConfigProviderFromData(`
[federation]
ENABLED = true
[service]
EMAIL_DOMAIN_BLOCKLIST = *.block.random
`)

	require.NoError(t, err)
	loadServerFrom(cfg)
	loadFederationFrom(cfg)
	loadServiceFrom(cfg)

	assert.Empty(t, Service.EmailDomainAllowList)
}

func TestAppVersionDocsURL(t *testing.T) {
	cases := [][3]string{
		{"next", "/", "https://forgejo.org/docs/next/"},
		{"latest", "/", "https://forgejo.org/docs/latest/"},
		// version ignores leading/trailing slashes:
		{"/latest", "/", "https://forgejo.org/docs/latest/"},
		{"/latest/", "/", "https://forgejo.org/docs/latest/"},
		{"latest/", "/", "https://forgejo.org/docs/latest/"},
		// version takes middle slashes verbatim:
		{"lat/est", "", "https://forgejo.org/docs/lat/est/"},
		{"/lat/est", "", "https://forgejo.org/docs/lat/est/"},
		{"/lat/est/", "", "https://forgejo.org/docs/lat/est/"},
		{"lat/est/", "", "https://forgejo.org/docs/lat/est/"},
		{"lat/est/", "foo", "https://forgejo.org/docs/lat/est/foo"},
		// empty path is acceptable:
		{"latest", "", "https://forgejo.org/docs/latest/"},
		{"latest", "//", "https://forgejo.org/docs/latest/"},
		{"latest", "/user/", "https://forgejo.org/docs/latest/user/"},
		// optional leading slash in path:
		{"latest", "user/", "https://forgejo.org/docs/latest/user/"},
		// verbatim trailing slash in path:
		{"latest", "user", "https://forgejo.org/docs/latest/user"},
		{"latest", "あ", "https://forgejo.org/docs/latest/%E3%81%82"},
		{"latest", "/あ", "https://forgejo.org/docs/latest/%E3%81%82"},
		{"latest", "/あ/", "https://forgejo.org/docs/latest/%E3%81%82/"},
		{"latest", "あ/", "https://forgejo.org/docs/latest/%E3%81%82/"},
		{"latest", "/user/#hash", "https://forgejo.org/docs/latest/user/#hash"},
		{"latest", "/user/#hash-hash", "https://forgejo.org/docs/latest/user/#hash-hash"},
		{"latest", "/user/#hash/hash", "https://forgejo.org/docs/latest/user/#hash/hash"},
		{"latest", "/user", "https://forgejo.org/docs/latest/user"},
		// valid escape sequences
		{"latest", "/user%2", "https://forgejo.org/docs/latest/user%252"},
		{"latest", "/user%%", "https://forgejo.org/docs/latest/user%25%25"},
		{"lat%2est", "/user%%", "https://forgejo.org/docs/lat%252est/user%25%25"},
		{"lat%%est", "/user%%", "https://forgejo.org/docs/lat%25%25est/user%25%25"},
		{"lat/%%est", "/user%%", "https://forgejo.org/docs/lat/%25%25est/user%25%25"},
		// verbatim hash
		{"latest", "/user#hash", "https://forgejo.org/docs/latest/user#hash"},
		{"latest", "/user#hash-hash", "https://forgejo.org/docs/latest/user#hash-hash"},
		{"latest", "/user#hash/hash", "https://forgejo.org/docs/latest/user#hash/hash"},
	}
	for _, c := range cases {
		version := c[0]
		path := c[1]
		expectedURL := c[2]

		t.Run(version+", "+path, func(t *testing.T) {
			assert.Equal(t, expectedURL, AppVersionDocsURL(version, path))
		})
	}
}

func TestAppDocsURL(t *testing.T) {
	defer test.MockProtect(&AppVer)()
	defer test.MockVariableValue(&AppDocsVer, initialAppDocsVer)() // instead of OnceValue

	cases := [][2]string{
		{"", "https://forgejo.org/docs/latest/user/getting-started/first-repository/"},
		{"dev", "https://forgejo.org/docs/latest/user/getting-started/first-repository/"},
		{"foo", "https://forgejo.org/docs/latest/user/getting-started/first-repository/"},
		{"あ", "https://forgejo.org/docs/latest/user/getting-started/first-repository/"},
		{"0.", "https://forgejo.org/docs/latest/user/getting-started/first-repository/"},
		{"0.zero.four", "https://forgejo.org/docs/latest/user/getting-started/first-repository/"},
		{"0.0.four", "https://forgejo.org/docs/latest/user/getting-started/first-repository/"},
		{"0.0.0", "https://forgejo.org/docs/v0.0/user/getting-started/first-repository/"},
		{"0.0.1", "https://forgejo.org/docs/v0.0/user/getting-started/first-repository/"},
		{"0.0.1", "https://forgejo.org/docs/v0.0/user/getting-started/first-repository/"},
		{"0.0", "https://forgejo.org/docs/v0.0/user/getting-started/first-repository/"},
		{"0", "https://forgejo.org/docs/v0.0/user/getting-started/first-repository/"},
		{"17.0.0", "https://forgejo.org/docs/v17.0/user/getting-started/first-repository/"},
		{"17.0.9", "https://forgejo.org/docs/v17.0/user/getting-started/first-repository/"},
		{"17.1.9", "https://forgejo.org/docs/v17.1/user/getting-started/first-repository/"},
		{"    17.1.9", "https://forgejo.org/docs/v17.1/user/getting-started/first-repository/"},
		{"17.1.9+prerelease", "https://forgejo.org/docs/v17.1/user/getting-started/first-repository/"},
		{"16.0.0-dev-753-6bcc6da0+gitea-1.22.0", "https://forgejo.org/docs/v16.0/user/getting-started/first-repository/"},
	}
	for _, c := range cases {
		appVersion := c[0]
		expectedURL := c[1]

		t.Run(appVersion, func(t *testing.T) {
			AppVer = appVersion
			assert.Equal(t, expectedURL, AppDocsURL("user/getting-started/first-repository/"))
		})
	}
}
