// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forgejo.org/models/db"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectPublicKeys(t *testing.T) {
	defer test.MockVariableValue(&setting.SSH.RootPath, t.TempDir())()
	authorizedKeysPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")

	t.Run("Missing", func(t *testing.T) {
		findings, err := InspectPublicKeys(t.Context())
		require.NoError(t, err)
		require.Len(t, findings, 1)
		f := findings[0]
		assert.Equal(t, InspectionResultFileMissing, f.Type)
	})

	t.Run("Generated cleanly", func(t *testing.T) {
		err := RewriteAllPublicKeys(t.Context())
		require.NoError(t, err)
		findings, err := InspectPublicKeys(t.Context())
		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("Extra unexpected key", func(t *testing.T) {
		err := RewriteAllPublicKeys(t.Context())
		require.NoError(t, err)

		file, err := os.OpenFile(authorizedKeysPath, os.O_WRONLY|os.O_APPEND, 0o600)
		require.NoError(t, err)
		_, err = file.WriteString("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHnbqnHNKh/Td/1O6t9Q8qdJmitCAApnvHImHV8TkptX hacker@example.com\n")
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		findings, err := InspectPublicKeys(t.Context())
		require.NoError(t, err)
		require.Len(t, findings, 1)
		f := findings[0]
		assert.Equal(t, InspectionResultUnexpectedKey, f.Type)
		assert.Contains(t, f.Comment, "Unexpected key on line")
	})

	t.Run("Missing expected key", func(t *testing.T) {
		file, err := os.OpenFile(authorizedKeysPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o600)
		require.NoError(t, err)
		err = db.GetEngine(t.Context()).Where("type != ?", KeyTypePrincipal).Iterate(new(PublicKey), func(idx int, bean any) (err error) {
			if idx == 0 {
				// Skip one key
				return nil
			}
			_, err = file.WriteString(bean.(*PublicKey).AuthorizedString())
			return err
		})
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)

		findings, err := InspectPublicKeys(t.Context())
		require.NoError(t, err)
		require.Len(t, findings, 1)
		f := findings[0]
		assert.Equal(t, InspectionResultMissingExpectedKey, f.Type)
		assert.Contains(t, f.Comment, "Key in database is not present in")
	})
}

func TestRewriteAllPublicKeys(t *testing.T) {
	defer test.MockVariableValue(&setting.SSH.RootPath, t.TempDir())()
	authorizedKeysPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")

	t.Run("Generated cleanly", func(t *testing.T) {
		err := RewriteAllPublicKeys(t.Context())
		require.NoError(t, err)

		count, err := db.GetEngine(t.Context()).Where("type != ?", KeyTypePrincipal).Count(&PublicKey{})
		require.NoError(t, err)

		content, err := os.ReadFile(authorizedKeysPath)
		require.NoError(t, err)
		stringContent := string(content)

		lines := strings.Split(stringContent, "\n")
		assert.Len(t, lines, int((count*2)+1)) // one comment + one key for each key (*2), plus a newline at the end (+1)
	})

	for _, allowUnexpectedAuthorizedKeys := range []bool{true, false} {
		t.Run(fmt.Sprintf("AllowUnexpectedAuthorizedKeys=%v", allowUnexpectedAuthorizedKeys), func(t *testing.T) {
			defer test.MockVariableValue(&setting.SSH.AllowUnexpectedAuthorizedKeys, allowUnexpectedAuthorizedKeys)()

			err := RewriteAllPublicKeys(t.Context())
			require.NoError(t, err)

			file, err := os.OpenFile(authorizedKeysPath, os.O_WRONLY|os.O_APPEND, 0o600)
			require.NoError(t, err)
			_, err = file.WriteString("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHnbqnHNKh/Td/1O6t9Q8qdJmitCAApnvHImHV8TkptX hacker@example.com\n")
			require.NoError(t, err)
			err = file.Close()
			require.NoError(t, err)

			err = RewriteAllPublicKeys(t.Context())
			require.NoError(t, err)

			content, err := os.ReadFile(authorizedKeysPath)
			require.NoError(t, err)
			stringContent := string(content)
			if allowUnexpectedAuthorizedKeys {
				assert.Contains(t, stringContent, "hacker@example.com")
			} else {
				assert.NotContains(t, stringContent, "hacker@example.com")
			}
		})
	}
}
