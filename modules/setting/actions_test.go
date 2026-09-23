// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"path/filepath"
	"testing"

	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func storageType(defaultStorageType, override string) string {
	storageType := defaultStorageType
	if storageType == "" {
		storageType = "local"
	}
	if override != "" {
		storageType = "minio"
	}
	return storageType
}

func assertActionsLog(t *testing.T, storageType string) {
	assert.EqualValues(t, storageType, Actions.LogStorage.Type)
	if storageType == "local" {
		assert.Equal(t, "actions_log", filepath.Base(Actions.LogStorage.Path))
	} else if storageType == "minio" {
		assert.Equal(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)
	} else {
		panic("test bug")
	}
}

func assertActionsArtifacts(t *testing.T, storageType string) {
	assert.EqualValues(t, storageType, Actions.ArtifactStorage.Type)
	if storageType == "local" {
		assert.Equal(t, "actions_artifacts", filepath.Base(Actions.ArtifactStorage.Path))
	} else if storageType == "minio" {
		assert.Equal(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)
	} else {
		panic("test bug")
	}
}

type actionsStorageCase struct {
	defaultStorageType      string
	actionsLogStorage       string
	actionsArtifactsStorage string
}

func Test_getStorageInheritNameSectionTypeForActions(t *testing.T) {
	defaultStorageTypes := []string{"", "local", "minio"}
	actionsLogStorages := []string{"", "local", "minio", "mystorageA", "mystorageB"}
	actionsArtifactsStorages := []string{"", "local", "minio", "mystorageA", "mystorageB"}

	// for clarity and to reduce nesting, build cartesian product first
	var tCase []actionsStorageCase
	for _, defaultStorageType := range defaultStorageTypes {
		for _, actionsLogStorage := range actionsLogStorages {
			for _, actionsArtifactsStorage := range actionsArtifactsStorages {
				tCase = append(tCase, actionsStorageCase{defaultStorageType, actionsLogStorage, actionsArtifactsStorage})
			}
		}
	}

	for _, c := range tCase {
		iniStr := ""

		if c.defaultStorageType != "" {
			iniStr += fmt.Sprintf("[storage]\nSTORAGE_TYPE = %s\n", c.defaultStorageType)
		}
		if c.actionsLogStorage != "" {
			iniStr += fmt.Sprintf("[storage.%s]\nSTORAGE_TYPE = %s\n", "actions_log", c.actionsLogStorage)
		}
		if c.actionsArtifactsStorage != "" {
			iniStr += fmt.Sprintf("[storage.%s]\nSTORAGE_TYPE = %s\n", "actions_artifacts", c.actionsArtifactsStorage)
		}
		if c.actionsLogStorage != "" {
			iniStr += fmt.Sprintf("[storage.%s]\nSTORAGE_TYPE = minio\n", c.actionsLogStorage)
		}
		if c.actionsArtifactsStorage != "" && c.actionsLogStorage != c.actionsArtifactsStorage {
			iniStr += fmt.Sprintf("[storage.%s]\nSTORAGE_TYPE = minio\n", c.actionsArtifactsStorage)
		}
		t.Run(fmt.Sprintf("%q.%q.%q", c.defaultStorageType, c.actionsLogStorage, c.actionsArtifactsStorage), func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(iniStr)
			require.NoError(t, err)
			require.NoError(t, loadActionsFrom(cfg))
			t.Log("\n" + iniStr)
			assertActionsLog(t, storageType(c.defaultStorageType, c.actionsLogStorage))
			assertActionsArtifacts(t, storageType(c.defaultStorageType, c.actionsArtifactsStorage))
		})
	}
}

func Test_getDefaultActionsURLForActions(t *testing.T) {
	oldActions := Actions
	oldAppURL := AppURL
	defer func() {
		Actions = oldActions
		AppURL = oldAppURL
	}()

	AppURL = "http://test_get_default_actions_url_for_actions:3000/"

	tests := []struct {
		name    string
		iniStr  string
		wantURL string
	}{
		{
			name: "default",
			iniStr: `
[actions]
`,
			wantURL: "https://data.forgejo.org",
		},
		{
			name: "github",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = github
`,
			wantURL: "https://github.com",
		},
		{
			name: "self",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = self
`,
			wantURL: "http://test_get_default_actions_url_for_actions:3000",
		},
		{
			name: "custom urls",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = https://example.com
`,
			wantURL: "https://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(tt.iniStr)
			require.NoError(t, err)
			require.NoError(t, loadActionsFrom(cfg))

			assert.Equal(t, tt.wantURL, Actions.DefaultActionsURL.URL())
		})
	}
}

func Test_getIDTokenSettingsForActions(t *testing.T) {
	defer test.MockVariableValue(&AppDataPath, "/home/app/data")()

	oldActions := Actions
	oldAppURL := AppURL
	defer func() {
		Actions = oldActions
		AppURL = oldAppURL
	}()

	iniStr := `
  [actions]
  `
	cfg, err := NewConfigProviderFromData(iniStr)
	require.NoError(t, err)
	require.NoError(t, loadActionsFrom(cfg))

	assert.Equal(t, "RS256", Actions.IDTokenKeyCfg.Signing.Algorithm)
	assert.Equal(t, "/home/app/data/actions_id_token/private.pem", *Actions.IDTokenKeyCfg.Signing.PrivateKeyPath)
	assert.EqualValues(t, 3600, Actions.IDTokenExpirationTime)

	iniStr = `
  [actions]
	ID_TOKEN_SIGNING_ALGORITHM = ES256
  ID_TOKEN_SIGNING_PRIVATE_KEY_FILE = /test/test.pem
  ID_TOKEN_EXPIRATION_TIME = 120
	`
	cfg, err = NewConfigProviderFromData(iniStr)
	require.NoError(t, err)
	require.NoError(t, loadActionsFrom(cfg))

	assert.Equal(t, "ES256", Actions.IDTokenKeyCfg.Signing.Algorithm)
	assert.Equal(t, "/test/test.pem", *Actions.IDTokenKeyCfg.Signing.PrivateKeyPath)
	assert.EqualValues(t, 120, Actions.IDTokenExpirationTime)

	iniStr = `
  [actions]
	ID_TOKEN_SIGNING_ALGORITHM = EdDSA
  ID_TOKEN_SIGNING_PRIVATE_KEY_FILE = ./test/test.pem
  ID_TOKEN_EXPIRATION_TIME = 123
	`
	cfg, err = NewConfigProviderFromData(iniStr)
	require.NoError(t, err)
	require.NoError(t, loadActionsFrom(cfg))

	assert.Equal(t, "EdDSA", Actions.IDTokenKeyCfg.Signing.Algorithm)
	assert.Equal(t, "/home/app/data/test/test.pem", *Actions.IDTokenKeyCfg.Signing.PrivateKeyPath)
	assert.EqualValues(t, 123, Actions.IDTokenExpirationTime)

	iniStr = `
  [actions]
	ID_TOKEN_SIGNING_ALGORITHM = HS256
	`
	cfg, err = NewConfigProviderFromData(iniStr)
	require.NoError(t, err)
	err = loadActionsFrom(cfg)
	require.ErrorContains(t, err, "[actions] Unexpected algorithm: ID_TOKEN_SIGNING_ALGORITHM = HS256, needs to be one of [RS256 RS384 RS512 ES256 ES384 ES512 EdDSA]")

	iniStr = `
  [actions]
	ID_TOKEN_SECRET = ABC
	`
	cfg, err = NewConfigProviderFromData(iniStr)
	require.NoError(t, err)
	err = loadActionsFrom(cfg)
	require.ErrorContains(t, err, "[actions] Invalid config key: ID_TOKEN_SECRET - must be removed")

	iniStr = `
  [actions]
	ID_TOKEN_SECRET_URI = ABC
	`
	cfg, err = NewConfigProviderFromData(iniStr)
	require.NoError(t, err)
	err = loadActionsFrom(cfg)
	require.ErrorContains(t, err, "[actions] Invalid config key: ID_TOKEN_SECRET_URI - must be removed")

	iniStr = `
  [actions]
	ID_TOKEN_KEYS_ACCEPTED = HS384:ForgejoForgejoForgejoForgejoForgejoForgejo_
	`
	cfg, err = NewConfigProviderFromData(iniStr)
	require.NoError(t, err)
	err = loadActionsFrom(cfg)
	require.ErrorContains(t, err, "[actions] Unexpected algorithm: ID_TOKEN_KEYS_ACCEPTED = HS384, needs to be one of [RS256 RS384 RS512 ES256 ES384 ES512 EdDSA]")
}
