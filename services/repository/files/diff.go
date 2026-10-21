// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"strings"

	repo_model "forgejo.org/models/repo"
	"forgejo.org/modules/lfs"
	"forgejo.org/modules/util"
	"forgejo.org/services/gitdiff"
)

// GetDiffPreview produces and returns diff result of a file which is not yet committed.
func GetDiffPreview(ctx context.Context, repo *repo_model.Repository, baseBranch, treePath, content string) (*gitdiff.Diff, error) {
	if baseBranch == "" {
		baseBranch = repo.DefaultBranch
	}
	t, err := NewTemporaryUploadRepository(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if err := t.Clone(baseBranch, true); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return nil, err
	}

	// Add the object to the database
	objectHash, err := t.HashObject(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Add the object to the index
	if err := t.AddObjectToIndex("100644", objectHash, treePath); err != nil {
		return nil, err
	}
	diff, err := t.DiffIndex()
	if err != nil {
		return diff, err
	}

	// This is a hack intended to add support for parsing potential Git LFS
	// pointers at the lowest level possible without access to a 'head' and 'base'
	// commit. Just like the diff preview visible when editing a file over the web
	// UI, it only supports one file for now.
	//
	// TODO: Potentially support Git LFS diffs once 'getFileReader' (from which
	// we copied and pasted from) is refactored out.
	if len(diff.Files) == 1 {
		blob, err := t.gitRepo.GetBlob(objectHash)
		if err != nil {
			return diff, err
		}

		buf := make([]byte, lfs.BlobSizeCutoff)
		dataRc, err := blob.DataAsync()
		if err != nil {
			return diff, err
		}
		defer dataRc.Close()
		n, err := util.ReadAtMost(dataRc, buf)
		if err != nil {
			return diff, err
		}
		buf = buf[:n]
		_, err = lfs.ReadPointerFromBuffer(buf)
		if err == nil {
			diff.Files[0].IsLFSFile = true
		}
	}
	return diff, nil
}
