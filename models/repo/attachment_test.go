// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"cmp"
	"slices"
	"testing"

	"forgejo.org/models/db"
	repo_model "forgejo.org/models/repo"
	"forgejo.org/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncreaseDownloadCount(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Release", func(t *testing.T) {
		attachment, err := repo_model.GetAttachmentByUUID(db.DefaultContext, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a19")
		require.NoError(t, err)
		assert.Equal(t, int64(0), attachment.DownloadCount)

		// increase download count
		err = attachment.IncreaseDownloadCount(db.DefaultContext)
		require.NoError(t, err)

		attachment, err = repo_model.GetAttachmentByUUID(db.DefaultContext, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a19")
		require.NoError(t, err)
		assert.Equal(t, int64(1), attachment.DownloadCount)
	})

	t.Run("Other attachment", func(t *testing.T) {
		attachment, err := repo_model.GetAttachmentByUUID(db.DefaultContext, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
		require.NoError(t, err)
		assert.Equal(t, int64(0), attachment.DownloadCount)

		// increase download count
		err = attachment.IncreaseDownloadCount(db.DefaultContext)
		require.NoError(t, err)

		attachment, err = repo_model.GetAttachmentByUUID(db.DefaultContext, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
		require.NoError(t, err)
		assert.Equal(t, int64(0), attachment.DownloadCount)
	})
}

func TestGetByCommentOrIssueID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// count of attachments from issue ID
	attachments, err := repo_model.GetAttachmentsByIssueID(db.DefaultContext, 1)
	require.NoError(t, err)
	assert.Len(t, attachments, 1)

	attachments, err = repo_model.GetAttachmentsByCommentID(db.DefaultContext, 1)
	require.NoError(t, err)
	assert.Len(t, attachments, 2)
}

func TestDeleteAttachments(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	count, err := repo_model.DeleteAttachmentsByComment(db.DefaultContext, 2, false)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	err = repo_model.DeleteAttachment(db.DefaultContext, &repo_model.Attachment{ID: 8}, false)
	require.NoError(t, err)

	attachment, err := repo_model.GetAttachmentByUUID(db.DefaultContext, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a18")
	require.Error(t, err)
	assert.True(t, repo_model.IsErrAttachmentNotExist(err))
	assert.Nil(t, attachment)
}

func TestGetAttachmentByID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	attach, err := repo_model.GetAttachmentByID(db.DefaultContext, 1)
	require.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)
}

func TestAttachment_DownloadURL(t *testing.T) {
	attach := &repo_model.Attachment{
		UUID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
		ID:   1,
	}
	assert.Equal(t, "https://try.gitea.io/attachments/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.DownloadURL())
}

func TestUpdateAttachment(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	attach, err := repo_model.GetAttachmentByID(db.DefaultContext, 1)
	require.NoError(t, err)
	assert.Equal(t, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", attach.UUID)

	attach.Name = "new_name"
	require.NoError(t, repo_model.UpdateAttachment(db.DefaultContext, attach))

	unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{Name: "new_name"})
}

func TestFindRepoAttachmentsByUUID(t *testing.T) {
	defer unittest.OverrideFixtures("models/repo/fixtures/TestFindRepoAttachmentsByUUID")()
	require.NoError(t, unittest.PrepareTestDatabase())

	sort := func(x []*repo_model.Attachment) {
		slices.SortFunc(x, func(a, b *repo_model.Attachment) int {
			return cmp.Compare(a.ID, b.ID)
		})
	}

	t.Run("Empty UUIDs", func(t *testing.T) {
		attachments, err := repo_model.FindRepoAttachmentsByUUID(t.Context(), 1001, []string{}, repo_model.FindAttachmentOptions{})
		require.NoError(t, err)
		assert.Empty(t, attachments)
	})

	t.Run("Wrong repository", func(t *testing.T) {
		attachments, err := repo_model.FindRepoAttachmentsByUUID(t.Context(), 1002, []string{"31b6f65e-2745-4e87-b02c-e6bb9890d399", "e19fd169-c2d1-4fd0-a6d5-9658fd4affed", "758e41f6-e3b7-4420-b34f-1920da0858aa"}, repo_model.FindAttachmentOptions{})
		require.NoError(t, err)
		assert.Empty(t, attachments)
	})

	t.Run("Not attached", func(t *testing.T) {
		attachments, err := repo_model.FindRepoAttachmentsByUUID(t.Context(), 1001, []string{"31b6f65e-2745-4e87-b02c-e6bb9890d399", "e19fd169-c2d1-4fd0-a6d5-9658fd4affed", "758e41f6-e3b7-4420-b34f-1920da0858aa"}, repo_model.FindAttachmentOptions{})
		require.NoError(t, err)
		if assert.Len(t, attachments, 1) {
			assert.Equal(t, "31b6f65e-2745-4e87-b02c-e6bb9890d399", attachments[0].UUID)
		}
	})

	t.Run("Issue", func(t *testing.T) {
		attachments, err := repo_model.FindRepoAttachmentsByUUID(t.Context(), 1001, []string{"17bcdb6b-dd84-4da1-b37a-671165402d8d", "e19fd169-c2d1-4fd0-a6d5-9658fd4affed", "774f276e-c85d-488e-b735-7bc07860c756"}, repo_model.FindAttachmentOptions{IssueID: 1001})
		require.NoError(t, err)
		sort(attachments)
		if assert.Len(t, attachments, 2) {
			assert.Equal(t, "17bcdb6b-dd84-4da1-b37a-671165402d8d", attachments[0].UUID)
			assert.Equal(t, "e19fd169-c2d1-4fd0-a6d5-9658fd4affed", attachments[1].UUID)
		}
	})

	t.Run("Comment", func(t *testing.T) {
		attachments, err := repo_model.FindRepoAttachmentsByUUID(t.Context(), 1001, []string{"edf0d986-8a12-447a-a4bb-e9aefead251b", "774f276e-c85d-488e-b735-7bc07860c756", "e19fd169-c2d1-4fd0-a6d5-9658fd4affed"}, repo_model.FindAttachmentOptions{IssueID: 1001, CommentID: 1001})
		require.NoError(t, err)
		if assert.Len(t, attachments, 1) {
			assert.Equal(t, "edf0d986-8a12-447a-a4bb-e9aefead251b", attachments[0].UUID)
		}

		attachments, err = repo_model.FindRepoAttachmentsByUUID(t.Context(), 1001, []string{"edf0d986-8a12-447a-a4bb-e9aefead251b", "774f276e-c85d-488e-b735-7bc07860c756", "e19fd169-c2d1-4fd0-a6d5-9658fd4affed"}, repo_model.FindAttachmentOptions{IssueID: 1001, CommentID: 1002})
		require.NoError(t, err)
		if assert.Len(t, attachments, 1) {
			assert.Equal(t, "774f276e-c85d-488e-b735-7bc07860c756", attachments[0].UUID)
		}
	})

	t.Run("Release", func(t *testing.T) {
		attachments, err := repo_model.FindRepoAttachmentsByUUID(t.Context(), 1001, []string{"d2570bab-c843-486f-b7b7-23e011c42815", "758e41f6-e3b7-4420-b34f-1920da0858aa", "e19fd169-c2d1-4fd0-a6d5-9658fd4affed"}, repo_model.FindAttachmentOptions{ReleaseID: 1001})
		require.NoError(t, err)
		if assert.Len(t, attachments, 2) {
			sort(attachments)
			assert.Equal(t, "758e41f6-e3b7-4420-b34f-1920da0858aa", attachments[0].UUID)
			assert.Equal(t, "d2570bab-c843-486f-b7b7-23e011c42815", attachments[1].UUID)
		}
	})
}
