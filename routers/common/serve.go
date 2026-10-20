// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"io"
	"time"

	git_model "forgejo.org/models/git"
	"forgejo.org/modules/git"
	"forgejo.org/modules/httpcache"
	"forgejo.org/modules/httplib"
	"forgejo.org/modules/lfs"
	"forgejo.org/modules/log"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/storage"
	"forgejo.org/services/context"
)

// ServeBlob serves a [git.Blob] (redirects to LFS if necessary,
// with caching support).
//
// Git LFS pointers are resolved. See also: [ServeBlobRaw]
func ServeBlob(ctx *context.Base, repo *context.Repository, blob *git.Blob, lastModified *time.Time) error {
	// Deal with caching (blob ID).
	if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
		return nil
	}

	// If the blob is bigger than 1023 bytes, it can't be an LFS pointer
	// that we can derive an LFS file from.
	if blob.Size() >= lfs.BlobSizeCutoff {
		return ServeBlobRaw(ctx, repo, blob, lastModified)
	}

	// Now that we don't know whether this is an LFS file or not,
	// let's investigate...
	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}

	// There are multiple steps involved in resolving an LFS file, namely
	// parsing the blob to check whether it contains a Git LFS pointer, and if so,
	// try to read the file from the LFS content store and serve that. If either
	// the parsing or the reading does not succeed, then the blob must represent
	// a "normal" file; so we check the cache for the given blob and, if nothing
	// is cached, we deallocate the buffer that is in-use for the blob and only
	// then do we "serve the blob" as-is.

	pointer, err := lfs.ReadPointer(dataRc)

	if err2 := dataRc.Close(); err2 != nil {
		log.Error("ServeBlob: Close: %v", err2)
		return err2
	}

	if err != nil {
		return ServeBlobRaw(ctx, repo, blob, lastModified)
	}

	// Now check if there is a MetaObject for this pointer
	meta, err := git_model.GetLFSMetaObjectByOid(ctx, repo.Repository.ID, pointer.Oid)
	// If there isn't one, just serve the data directly
	if err != nil {
		if err == git_model.ErrLFSObjectNotExist || err == lfs.ErrInvalidOIDFormat {
			return ServeBlobRaw(ctx, repo, blob, lastModified)
		}

		return err
	}

	// Handle caching for the LFS object OID
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+pointer.Oid+`"`) {
		return nil
	}

	if setting.LFS.Storage.MinioConfig.ServeDirect {
		// If we have a signed url (S3, object storage, blob storage), redirect to this directly.
		u, err := storage.LFS.URL(pointer.RelativePath(), blob.Name(), nil)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return nil
		}
	}

	lfsDataRc, err := lfs.ReadMetaObject(meta.Pointer)
	if err != nil {
		return err
	}
	defer func() {
	}()

	ServeContentByReadSeeker(ctx, repo.TreePath, lastModified, lfsDataRc)
	if err = lfsDataRc.Close(); err != nil {
		log.Error("ServeBlob: Close: %v", err)
	}
	return err
}

// ServeBlobRaw serves a [git.Blob] as-is (with caching support).
// For "Git LFS files", the file's pointer is served instead of its target.
// See also: [ServeBlob]
func ServeBlobRaw(ctx *context.Base, repo *context.Repository, blob *git.Blob, lastModified *time.Time) error {
	if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
		return nil
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}

	ServeContentByReader(ctx, repo.TreePath, blob.Size(), dataRc)
	if err = dataRc.Close(); err != nil {
		log.Error("ServeBlobRaw: Close: %v", err)
	}
	return err
}

func ServeContentByReader(ctx *context.Base, filePath string, size int64, reader io.Reader) {
	httplib.ServeContentByReader(ctx.Req, ctx.Resp, filePath, size, reader)
}

func ServeContentByReadSeeker(ctx *context.Base, filePath string, modTime *time.Time, reader io.ReadSeeker) {
	httplib.ServeContentByReadSeeker(ctx.Req, ctx.Resp, filePath, modTime, reader)
}
