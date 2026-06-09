// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bytes"
	"encoding/base64"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoIsEmpty(t *testing.T) {
	emptyRepo2Path := filepath.Join(testReposDir, "repo2_empty")
	repo, err := openRepositoryWithDefaultContext(emptyRepo2Path)
	require.NoError(t, err)
	defer repo.Close()
	isEmpty, err := repo.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty)
}

func TestRepoGetDivergingCommits(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	do, err := GetDivergingCommits(t.Context(), bareRepo1Path, "master", "branch2", nil)
	require.NoError(t, err)
	assert.Equal(t, DivergeObject{
		Ahead:  1,
		Behind: 5,
	}, do)

	do, err = GetDivergingCommits(t.Context(), bareRepo1Path, "master", "master", nil)
	require.NoError(t, err)
	assert.Equal(t, DivergeObject{
		Ahead:  0,
		Behind: 0,
	}, do)

	do, err = GetDivergingCommits(t.Context(), bareRepo1Path, "master", "test", nil)
	require.NoError(t, err)
	assert.Equal(t, DivergeObject{
		Ahead:  0,
		Behind: 2,
	}, do)
}

func TestCloneCredentials(t *testing.T) {
	calledWithoutPassword := false
	credentialsFile := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/info/refs" {
			return
		}

		// Get basic authorization.
		auth, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Basic ")
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Forgejo"`)
			http.Error(w, "require credentials", http.StatusUnauthorized)
			return
		}

		rawAuth, err := base64.StdEncoding.DecodeString(auth)
		require.NoError(t, err)

		user, password, ok := bytes.Cut(rawAuth, []byte{':'})
		assert.True(t, ok)

		// First time around Git must try without password (password was removed from the clone URL to not appear as argument).
		if len(password) == 0 {
			assert.EqualValues(t, "oauth2", user)
			calledWithoutPassword = true

			w.Header().Set("WWW-Authenticate", `Basic realm="Forgejo"`)
			http.Error(w, "require credentials", http.StatusUnauthorized)
			return
		}

		assert.EqualValues(t, "oauth2", user)
		assert.EqualValues(t, "some_token", password)

		tmpDir := os.TempDir()

		// Verify that the credential store was used.
		files, err := fs.Glob(os.DirFS(tmpDir), "forgejo-clone-credentials-*")
		require.NoError(t, err)
		for _, fileName := range files {
			credentialsFile = filepath.Join(tmpDir, fileName)
			fileContent, err := os.ReadFile(credentialsFile)
			require.NoError(t, err)

			assert.True(t, bytes.Contains(fileContent, []byte(`http`)), string(fileContent))
		}
	}))

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	serverURL.User = url.UserPassword("oauth2", "some_token")

	require.NoError(t, Clone(t.Context(), serverURL.String(), t.TempDir(), CloneRepoOptions{}))

	assert.True(t, calledWithoutPassword)
	assert.NotEmpty(t, credentialsFile)

	// Check that the credential file is gone.
	_, err = os.Stat(credentialsFile)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestInitRepositoryWithNoTemplates(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		bare             bool
		objectFormatName string
	}{
		{
			name:             "Bare sha1 repo",
			bare:             true,
			objectFormatName: Sha1ObjectFormat.Name(),
		},
		{
			name:             "Non-bare sha1 repo",
			bare:             false,
			objectFormatName: Sha1ObjectFormat.Name(),
		},
		{
			name:             "Bare sha256 repo",
			bare:             true,
			objectFormatName: Sha256ObjectFormat.Name(),
		},
		{
			name:             "Non-bare sha256 repo",
			bare:             false,
			objectFormatName: Sha256ObjectFormat.Name(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsValidObjectFormat(tt.objectFormatName) {
				t.Skip("not supported")
				return
			}

			repoPath := t.TempDir()

			err := InitRepository(t.Context(), repoPath, tt.bare, tt.objectFormatName)
			require.NoError(t, err, "couldn't init repository")

			_, err = os.Stat(repoPath + "/hooks")
			require.ErrorIs(t, err, os.ErrNotExist, "hooks directory shouldn't exist")

			_, err = os.Stat(repoPath + "/description")
			require.ErrorIs(t, err, os.ErrNotExist, "description file shouldn't exist")

			_, err = os.Stat(repoPath + "/info")
			require.ErrorIs(t, err, os.ErrNotExist, "info directory shouldn't exist")
		})
	}
}
