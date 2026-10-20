// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"time"

	"forgejo.org/modules/git"
	"forgejo.org/routers/common"
	"forgejo.org/services/context"
)

func getBlobForEntry(ctx *context.Context) (*git.Blob, *time.Time) {
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetTreeEntryByPath", err)
		} else {
			ctx.ServerError("GetTreeEntryByPath", err)
		}
		return nil, nil
	}

	if entry.IsDir() || entry.IsSubmodule() {
		ctx.NotFound("getBlobForEntry", nil)
		return nil, nil
	}

	latestCommit, err := ctx.Repo.GitRepo.GetTreePathLatestCommit(ctx.Repo.Commit.ID.String(), ctx.Repo.TreePath)
	if err != nil {
		ctx.ServerError("GetTreePathLatestCommit", err)
		return nil, nil
	}
	lastModified := &latestCommit.Committer.When

	return entry.Blob(), lastModified
}

// SingleDownloadRaw facilitates the downloading of a served file using the
// repository path.
//
// For "Git LFS files", the pointer is provided instead of the pointer's target.
// See also: [SingleDownload]
func SingleDownloadRaw(ctx *context.Context) {
	blob, lastModified := getBlobForEntry(ctx)
	if blob == nil {
		return
	}

	if err := common.ServeBlobRaw(ctx.Base, ctx.Repo, blob, lastModified); err != nil {
		ctx.ServerError("ServeBlobRaw", err)
	}
}

// SingleDownload facilitates the downloading of a served file using the
// repository path.
//
// Git LFS pointers are resolved. See also: [SingleDownloadRaw]
func SingleDownload(ctx *context.Context) {
	blob, lastModified := getBlobForEntry(ctx)
	if blob == nil {
		return
	}

	if err := common.ServeBlob(ctx.Base, ctx.Repo, blob, lastModified); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}

// DownloadByIDRaw downloads a file using its SHA1 ID.
// For "Git LFS files", the pointer is provided instead of the pointer's target.
// See also: [DownloadByID]
func DownloadByIDRaw(ctx *context.Context) {
	blob, err := ctx.Repo.GitRepo.GetBlob(ctx.Params("sha"))
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetBlob", nil)
		} else {
			ctx.ServerError("GetBlob", err)
		}
		return
	}
	if err = common.ServeBlobRaw(ctx.Base, ctx.Repo, blob, nil); err != nil {
		ctx.ServerError("ServeBlobRaw", err)
	}
}

// DownloadByID downloads a file suing its SHA1 ID.
// Git LFS pointers shall be resolved; their target shall be provided.
// See also: [DownloadByIDRaw]
func DownloadByID(ctx *context.Context) {
	blob, err := ctx.Repo.GitRepo.GetBlob(ctx.Params("sha"))
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetBlob", nil)
		} else {
			ctx.ServerError("GetBlob", err)
		}
		return
	}
	if err = common.ServeBlob(ctx.Base, ctx.Repo, blob, nil); err != nil {
		ctx.ServerError("ServeBlob", err)
	}
}
