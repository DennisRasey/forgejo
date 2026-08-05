// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package git

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLineBlame(t *testing.T) {
	t.Run("SHA1", func(t *testing.T) {
		repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
		require.NoError(t, err)
		defer repo.Close()

		commit, lineno, err := repo.LineBlame("HEAD", "foo/link_short", 1)
		require.NoError(t, err)
		assert.Equal(t, "37991dec2c8e592043f47155ce4808d4580f9123", commit.ID.String())
		assert.EqualValues(t, 1, lineno)

		commit, lineno, err = repo.LineBlame("HEAD", "foo/link_short", 512)
		require.ErrorIs(t, err, ErrBlameFileNotEnoughLines)
		assert.Nil(t, commit)
		assert.Zero(t, lineno)

		commit, lineno, err = repo.LineBlame("HEAD", "non-existent/path", 512)
		require.ErrorIs(t, err, ErrBlameFileDoesNotExist)
		assert.Nil(t, commit)
		assert.Zero(t, lineno)
	})

	t.Run("SHA256", func(t *testing.T) {
		skipIfSHA256NotSupported(t)

		repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare_sha256"))
		require.NoError(t, err)
		defer repo.Close()

		commit, lineno, err := repo.LineBlame("HEAD", "foo/link_short", 1)
		require.NoError(t, err)
		assert.Equal(t, "6aae864a3d1d0d6a5be0cc64028c1e7021e2632b031fd8eb82afc5a283d1c3d1", commit.ID.String())
		assert.EqualValues(t, 1, lineno)

		commit, lineno, err = repo.LineBlame("HEAD", "foo/link_short", 512)
		require.ErrorIs(t, err, ErrBlameFileNotEnoughLines)
		assert.Nil(t, commit)
		assert.Zero(t, lineno)

		commit, lineno, err = repo.LineBlame("HEAD", "non-existent/path", 512)
		require.ErrorIs(t, err, ErrBlameFileDoesNotExist)
		assert.Nil(t, commit)
		assert.Zero(t, lineno)
	})

	t.Run("Moved line", func(t *testing.T) {
		test := func(t *testing.T, objectFormatName string) {
			t.Helper()
			tmpDir := t.TempDir()

			require.NoError(t, InitRepository(t.Context(), tmpDir, false, objectFormatName))

			gitRepo, err := OpenRepository(t.Context(), tmpDir)
			require.NoError(t, err)
			defer gitRepo.Close()

			require.NoError(t, os.WriteFile(path.Join(tmpDir, "ANSWER"), []byte("abba\n"), 0o666))
			require.NoError(t, AddChanges(tmpDir, true))
			require.NoError(t, CommitChanges(tmpDir, CommitChangesOptions{Message: "Favourite singer of everyone who follows a automata course"}))

			firstCommit, err := gitRepo.GetRefCommitID("HEAD")
			require.NoError(t, err)

			require.NoError(t, os.WriteFile(path.Join(tmpDir, "ANSWER"), append(bytes.Repeat([]byte("baba\n"), 9), []byte("abba\n")...), 0o666))
			require.NoError(t, AddChanges(tmpDir, true))
			require.NoError(t, CommitChanges(tmpDir, CommitChangesOptions{Message: "Now there's several of them"}))

			secondCommit, err := gitRepo.GetRefCommitID("HEAD")
			require.NoError(t, err)

			commit, lineno, err := gitRepo.LineBlame("HEAD", "ANSWER", 10)
			require.NoError(t, err)
			assert.Equal(t, firstCommit, commit.ID.String())
			assert.EqualValues(t, 1, lineno)

			rev, err := gitRepo.ReverseLineBlame(commit.ID.String(), "ANSWER", lineno, secondCommit)
			require.NoError(t, err)
			assert.Equal(t, secondCommit, rev.CommitID)
			assert.Equal(t, "ANSWER", rev.FilePath)
			assert.EqualValues(t, 10, rev.LineNumber)

			for i := range uint64(9) {
				commit, lineno, err = gitRepo.LineBlame("HEAD", "ANSWER", i+1)
				require.NoError(t, err)
				assert.Equal(t, secondCommit, commit.ID.String())
				assert.Equal(t, i+1, lineno)

				rev, err := gitRepo.ReverseLineBlame(commit.ID.String(), "ANSWER", lineno, secondCommit)
				require.NoError(t, err)
				assert.Equal(t, secondCommit, rev.CommitID)
				assert.Equal(t, "ANSWER", rev.FilePath)
				assert.Equal(t, i+1, rev.LineNumber)
			}
		}

		t.Run("SHA1", func(t *testing.T) {
			test(t, Sha1ObjectFormat.Name())
		})

		t.Run("SHA256", func(t *testing.T) {
			skipIfSHA256NotSupported(t)

			test(t, Sha256ObjectFormat.Name())
		})
	})
}

func TestReverseLineBlame(t *testing.T) {
	t.Run("single commit", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, InitRepository(t.Context(), tmpDir, false, Sha1ObjectFormat.Name()))

		gitRepo, err := OpenRepository(t.Context(), tmpDir)
		require.NoError(t, err)
		defer gitRepo.Close()

		require.NoError(t, os.WriteFile(path.Join(tmpDir, "file1.md"), []byte("abba\n"), 0o666))
		require.NoError(t, AddChanges(tmpDir, true))
		require.NoError(t, CommitChanges(tmpDir, CommitChangesOptions{Message: "abba spelt backwards"}))

		commit, err := gitRepo.GetRefCommitID("HEAD")
		require.NoError(t, err)

		blameCommit, lineno, err := gitRepo.LineBlame("HEAD", "file1.md", 1)
		require.NoError(t, err)
		assert.Equal(t, commit, blameCommit.ID.String())
		assert.EqualValues(t, 1, lineno)

		rev, err := gitRepo.ReverseLineBlame(commit, "file1.md", lineno, commit)
		require.NoError(t, err)
		assert.Equal(t, commit, rev.CommitID)
		assert.Equal(t, "file1.md", rev.FilePath)
		assert.EqualValues(t, 1, rev.LineNumber)
	})

	t.Run("move file", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, InitRepository(t.Context(), tmpDir, false, Sha1ObjectFormat.Name()))

		gitRepo, err := OpenRepository(t.Context(), tmpDir)
		require.NoError(t, err)
		defer gitRepo.Close()

		require.NoError(t, os.WriteFile(path.Join(tmpDir, "file1.md"), []byte("abba\n"), 0o666))
		require.NoError(t, AddChanges(tmpDir, true))
		require.NoError(t, CommitChanges(tmpDir, CommitChangesOptions{Message: "abba spelt backwards"}))

		firstCommit, err := gitRepo.GetRefCommitID("HEAD")
		require.NoError(t, err)

		require.NoError(t, os.Rename(path.Join(tmpDir, "file1.md"), path.Join(tmpDir, "file2.md")))
		require.NoError(t, AddChanges(tmpDir, true))
		require.NoError(t, CommitChanges(tmpDir, CommitChangesOptions{Message: "move file"}))

		secondCommit, err := gitRepo.GetRefCommitID("HEAD")
		require.NoError(t, err)

		blameCommit, lineno, err := gitRepo.LineBlame("HEAD", "file2.md", 1)
		require.NoError(t, err)
		assert.Equal(t, firstCommit, blameCommit.ID.String())
		assert.EqualValues(t, 1, lineno)

		rev, err := gitRepo.ReverseLineBlame(firstCommit, "file1.md", lineno, secondCommit)
		require.NoError(t, err)
		assert.Equal(t, secondCommit, rev.CommitID)
		assert.Equal(t, "file2.md", rev.FilePath)
		assert.EqualValues(t, 1, rev.LineNumber)
	})
}
