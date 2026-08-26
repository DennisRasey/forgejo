// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type indexerMatchList struct {
	value    string
	position int
}

func Test_newIndexerGlobSettings(t *testing.T) {
	checkGlobMatch(t, "", []indexerMatchList{})
	checkGlobMatch(t, "     ", []indexerMatchList{})
	checkGlobMatch(t, "data, */data, */data/*, **/data/*, **/data/**", []indexerMatchList{
		{"", -1},
		{"don't", -1},
		{"data", 0},
		{"/data", 1},
		{"x/data", 1},
		{"x/data/y", 2},
		{"a/b/c/data/z", 3},
		{"a/b/c/data/x/y/z", 4},
	})
	checkGlobMatch(t, "*.txt, txt, **.txt, **txt, **txt*", []indexerMatchList{
		{"my.txt", 0},
		{"don't", -1},
		{"mytxt", 3},
		{"/data/my.txt", 2},
		{"data/my.txt", 2},
		{"data/txt", 3},
		{"data/thistxtfile", 4},
		{"/data/thistxtfile", 4},
	})
	checkGlobMatch(t, "data/**/*.txt, data/**.txt", []indexerMatchList{
		{"data/a/b/c/d.txt", 0},
		{"data/a.txt", 1},
	})
	checkGlobMatch(t, "**/*.txt, data/**.txt", []indexerMatchList{
		{"data/a/b/c/d.txt", 0},
		{"data/a.txt", 0},
		{"a.txt", -1},
	})
}

func checkGlobMatch(t *testing.T, globstr string, list []indexerMatchList) {
	glist := IndexerGlobFromString(globstr)
	if len(list) == 0 {
		assert.Empty(t, glist)
		return
	}
	assert.NotEmpty(t, glist)
	for _, m := range list {
		found := false
		for pos, g := range glist {
			if g.Match(m.value) {
				assert.Equal(t, m.position, pos, "Test string `%s` doesn't match `%s`@%d, but matches @%d", m.value, globstr, m.position, pos)
				found = true
				break
			}
		}
		if !found {
			assert.Equal(t, -1, m.position, "Test string `%s` doesn't match `%s` anywhere; expected @%d", m.value, globstr, m.position)
		}
	}
}

// Possible cases:
// [http|https]://[[user]:pass@]host[:port][/[path]]
func Test_indexerURLConstruction(t *testing.T) {
	makeBaseConfig := func() (ConfigProvider, ConfigSection) {
		cfg, _ := NewConfigProviderFromData("")
		sec := cfg.Section("indexer")
		sec.NewKey("ISSUE_INDEXER_TYPE", "meilisearch")

		return cfg, sec
	}
	setConfigValues := func(sec ConfigSection, host, username, password, path, protocol string) {
		sec.NewKey("ISSUE_INDEXER_HOST", host)
		sec.NewKey("ISSUE_INDEXER_USER", username)
		sec.NewKey("ISSUE_INDEXER_PASSWD", password)
		sec.NewKey("ISSUE_INDEXER_PATH", path)
		sec.NewKey("ISSUE_INDEXER_PROTOCOL", protocol)
	}
	testCases := []struct {
		host     string
		username string
		password string
		path     string
		protocol string
		expected string
	}{
		{"host:80", "", "", "", "http", "http://host:80"},
		{"host", "", "", "/path", "http", "http://host/path"},
		{"host", "", "pass", "", "https", "https://:pass@host"},
		{"host", "user", "", "", "http", "http://user@host"},
		{"host:8080", "user", "pass", "/path", "http", "http://user:pass@host:8080/path"},
	}
	for _, test := range testCases {
		_, section := makeBaseConfig()
		setConfigValues(section, test.host, test.username, test.password, test.path, test.protocol)
		actual := buildConnectionString(section)
		assert.Equal(t, test.expected, actual)
	}

	t.Run("Uses ISSUE_INDEXER_PASSWD_URI", func(t *testing.T) {
		_, section := makeBaseConfig()

		password := "password"

		passwdURI := filepath.Join(t.TempDir(), "indexer_passwd")
		require.NoError(t, os.WriteFile(passwdURI, []byte(password), 0o644))

		section.NewKey("ISSUE_INDEXER_PROTOCOL", "http")
		section.NewKey("ISSUE_INDEXER_HOST", "host")
		section.NewKey("ISSUE_INDEXER_PASSWD_URI", fmt.Sprintf("file:%s", passwdURI))

		actual := buildConnectionString(section)

		assert.Equal(t, fmt.Sprintf("http://:%s@host", password), actual)
	})
	t.Run("Errors with connection string", func(t *testing.T) {
		_, section := makeBaseConfig()
		section.NewKey("ISSUE_INDEXER_CONN_STR", "http://user:pass@host:8080/path")
		setConfigValues(section, "newHost", "newUser", "pass", "/path", "http")
		_, err := getIndexerConnStr(section)
		assert.Error(t, err)
	})
}
