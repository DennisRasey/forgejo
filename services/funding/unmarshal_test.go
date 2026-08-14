// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPLv3-or-later

package funding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestCouldBeFundingConfig(t *testing.T) {
	cases := []string{
		".forgejo/FUNDING",
		".forgejo/Funding",
		".forgejo/funding",
		".forgejo/fundING",

		".github/FUNDING",
		".github/Funding",
		".github/funding",
		".github/fundING",

		"FUNDING",
		"Funding",
		"funding",
		"fundING",
	}
	for _, fileName := range cases {
		for _, extension := range []string{".yaml", ".yml", ".YML"} {
			path := fileName + extension
			t.Run(fileName, func(t *testing.T) {
				assert.True(t, CouldBeFundingConfig(path))
			})
		}
	}
}

func TestCannotBeFundingConfig(t *testing.T) {
	cases := []string{
		"README.md",
		".gitea/FUNDING.yml",
		"custom/FUNDING.yml",
		".forgejo/_FUNDING.yml",
		".forgejo/.FUNDING.yml",
		".forgejo/FUNDING.yml.",
		"",
	}
	for _, fileName := range cases {
		t.Run(fileName, func(t *testing.T) {
			assert.False(t, CouldBeFundingConfig(fileName))
		})
	}
}

func TestFundingConfigParse(t *testing.T) {
	// custom UnmarshalYAML implementation should let us gather ordering info on mapping pairs
	t.Run("no data", func(t *testing.T) {
		badConfigs := []string{
			"",
			"{}",
		}

		for _, configContent := range badConfigs {
			config := make(rawRepoFundingConfig, 0)
			err := yaml.Unmarshal([]byte(configContent), &config)
			require.NoError(t, err)
			assert.Empty(t, config)
		}
	})

	t.Run("bad data", func(t *testing.T) {
		badConfigs := []string{
			"42",
			"[42]",
			"['42']",
			"['']",
			"[]",
			"foo:bar",
			"あ",
		}

		for _, configContent := range badConfigs {
			config := make(rawRepoFundingConfig, 0)
			err := yaml.Unmarshal([]byte(configContent), &config)
			assert.Error(t, err, "config content: %v", configContent)
		}
	})

	t.Run("duplicate keys", func(t *testing.T) {
		configContent := "a: b\nfoo: bar\nfoo: baz"
		config := make(rawRepoFundingConfig, 0)
		err := yaml.Unmarshal([]byte(configContent), &config)
		assert.Error(t, err, "Duplicate YAML key: foo")
	})

	t.Run("dummy single data row", func(t *testing.T) {
		cases := [][2]any{
			{"foo: bar", rawRepoFundingConfigEntry{"foo", "bar"}},
			{"foo: [bar]", rawRepoFundingConfigEntry{"foo", []any{"bar"}}},
			{"foo: 42", rawRepoFundingConfigEntry{"foo", 42}},
			{"foo: [42]", rawRepoFundingConfigEntry{"foo", []any{42}}},
			{"foo:", rawRepoFundingConfigEntry{"foo", nil}},
			{"foo: 'localhost:4242'", rawRepoFundingConfigEntry{"foo", "localhost:4242"}},
			{"foo: 'localhost: 4242'", rawRepoFundingConfigEntry{"foo", "localhost: 4242"}},
			{"あ: '!'", rawRepoFundingConfigEntry{"あ", "!"}},
			{"あ:", rawRepoFundingConfigEntry{"あ", nil}},
			{`custom: '😀.com'`, rawRepoFundingConfigEntry{"custom", "😀.com"}},
			// adapted from the list at https://en.wikipedia.org/wiki/International_email:
			{"bar: '用户@例子.广告'", rawRepoFundingConfigEntry{"bar", "用户@例子.广告"}}, // at char is valid
			{"bar: 'ಡೇಟಾಮೇಲ್.ಭಾರತ'", rawRepoFundingConfigEntry{"bar", "ಡೇಟಾಮೇಲ್.ಭಾರತ"}},
			{"bar: 'डाटा.भारत'", rawRepoFundingConfigEntry{"bar", "डाटा.भारत"}},
			{"bar: 'пошта.укр'", rawRepoFundingConfigEntry{"bar", "пошта.укр"}},
			{"bar: 'παράδειγμα.ελ'", rawRepoFundingConfigEntry{"bar", "παράδειγμα.ελ"}},
			{"bar: 'Sörensen.example.com'", rawRepoFundingConfigEntry{"bar", "Sörensen.example.com"}},
			{"bar: 'пример.рф'", rawRepoFundingConfigEntry{"bar", "пример.рф"}},
			{"bar: 'موقع.عر'", rawRepoFundingConfigEntry{"bar", "موقع.عر"}},
		}

		for _, c := range cases {
			configContent := c[0].(string)
			expected := c[1].(rawRepoFundingConfigEntry)
			config := make(rawRepoFundingConfig, 0)
			err := yaml.Unmarshal([]byte(configContent), &config)
			require.NoError(t, err)
			assert.Len(t, config, 1)
			assert.Equal(t, expected, config[0])
		}
	})

	t.Run("orders a->b->c", func(t *testing.T) {
		configContent := "a: a\nb: b\nc: c"
		config := make(rawRepoFundingConfig, 0)
		err := yaml.Unmarshal([]byte(configContent), &config)
		require.NoError(t, err)
		assert.Len(t, config, 3)
		assert.Equal(t, rawRepoFundingConfigEntry{"a", "a"}, config[0])
		assert.Equal(t, rawRepoFundingConfigEntry{"b", "b"}, config[1])
		assert.Equal(t, rawRepoFundingConfigEntry{"c", "c"}, config[2])
	})

	t.Run("orders b->a->c", func(t *testing.T) {
		configContent := "b: b\na: a\nc: c"
		config := make(rawRepoFundingConfig, 0)
		err := yaml.Unmarshal([]byte(configContent), &config)
		require.NoError(t, err)
		assert.Len(t, config, 3)
		assert.Equal(t, rawRepoFundingConfigEntry{"b", "b"}, config[0])
		assert.Equal(t, rawRepoFundingConfigEntry{"a", "a"}, config[1])
		assert.Equal(t, rawRepoFundingConfigEntry{"c", "c"}, config[2])
	})

	t.Run("orders c->a->b", func(t *testing.T) {
		configContent := "c: c\na: a\nb: b"
		config := make(rawRepoFundingConfig, 0)
		err := yaml.Unmarshal([]byte(configContent), &config)
		require.NoError(t, err)
		assert.Len(t, config, 3)
		assert.Equal(t, rawRepoFundingConfigEntry{"c", "c"}, config[0])
		assert.Equal(t, rawRepoFundingConfigEntry{"a", "a"}, config[1])
		assert.Equal(t, rawRepoFundingConfigEntry{"b", "b"}, config[2])
	})

	t.Run("orders c->b->a", func(t *testing.T) {
		configContent := "c: c\nb: b\na: a"
		config := make(rawRepoFundingConfig, 0)
		err := yaml.Unmarshal([]byte(configContent), &config)
		require.NoError(t, err)
		assert.Len(t, config, 3)
		assert.Equal(t, rawRepoFundingConfigEntry{"c", "c"}, config[0])
		assert.Equal(t, rawRepoFundingConfigEntry{"b", "b"}, config[1])
		assert.Equal(t, rawRepoFundingConfigEntry{"a", "a"}, config[2])
	})
}
