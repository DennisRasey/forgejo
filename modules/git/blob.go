// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	"forgejo.org/modules/log"
	"forgejo.org/modules/typesniffer"
)

// Blob represents a Git object.
type Blob struct {
	ID ObjectID

	gotSize bool
	size    int64
	name    string
	repo    *Repository
}

func (b *Blob) newReader() (*bufio.Reader, int64, func(error), error) {
	wr, rd, cancel, err := b.repo.CatFileBatch(b.repo.Ctx)
	if err != nil {
		return nil, 0, nil, err
	}

	_, err = wr.Write([]byte(b.ID.String() + "\n"))
	if err != nil {
		cancel(err)
		return nil, 0, nil, err
	}
	_, _, size, err := ReadBatchLine(rd)
	if err != nil {
		cancel(err)
		return nil, 0, nil, err
	}
	b.gotSize = true
	b.size = size
	return rd, size, cancel, err
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	if b.gotSize {
		return b.size
	}

	if err := b.repo.WithCatFileBatchCheck(b.repo.Ctx, func(wr io.Writer, rd *bufio.Reader) error {
		_, err := wr.Write([]byte(b.ID.String() + "\n"))
		if err != nil {
			return nil
		}
		_, _, b.size, err = ReadBatchLine(rd)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}

	b.gotSize = true
	return b.size
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	rd, size, cancel, err := b.newReader()
	if err != nil {
		return nil, err
	}

	// note: size of this optimization is involved in unit tests, specifically TestExpandLocalReusableWorkflows -- if
	// the optimization is ever changed, the tests should be adjusted to ensure the non-optimized workflow is tested
	if size < 4096 {
		buf := make([]byte, size)
		_, err := io.ReadFull(rd, buf)
		defer func() { cancel(err) }()
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		return io.NopCloser(bytes.NewReader(buf)), err
	}

	return &blobReader{
		rd:     rd,
		n:      size,
		cancel: cancel,
	}, nil
}

type blobReader struct {
	rd                *bufio.Reader
	n                 int64 // number of bytes to read
	additionalDiscard int64 // additional number of bytes to discard
	cancel            func(error)
}

func (b *blobReader) Read(p []byte) (n int, err error) {
	if b.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.n {
		p = p[0:b.n]
	}
	n, err = b.rd.Read(p)
	b.n -= int64(n)
	return n, err
}

// Close implements io.Closer
func (b *blobReader) Close() error {
	if b.rd == nil {
		return nil
	}

	// discard the unread bytes, the truncated bytes and the trailing newline
	if err := DiscardFull(b.rd, b.n+b.additionalDiscard+1); err != nil {
		b.cancel(err)
		return err
	}

	b.cancel(nil)
	b.rd = nil

	return nil
}

// Name returns name of the tree entry this blob object was created from (or empty string)
func (b *Blob) Name() string {
	return b.name
}

// NewReader return a blob-reader which fails immediately with [BlobTooLargeError] if the file is bigger than the limit
func (b *Blob) NewReader(limit int64) (rc io.ReadCloser, actualSize int64, err error) {
	actualSize = b.Size()
	if actualSize > limit {
		return nil, actualSize, BlobTooLargeError{
			Size:  actualSize,
			Limit: limit,
		}
	}
	r, _, cancel, err := b.newReader()
	if err != nil {
		return nil, actualSize, err
	}

	return &blobReader{
		rd:                r,
		n:                 actualSize,
		additionalDiscard: 0,
		cancel:            cancel,
	}, actualSize, nil
}

// NewTruncatedReader return a blob-reader which silently truncates when the limit is reached (io.EOF will be returned)
func (b *Blob) NewTruncatedReader(limit int64) (rc io.ReadCloser, fullSize int64, err error) {
	r, fullSize, cancel, err := b.newReader()
	if err != nil {
		return nil, fullSize, err
	}

	limit = min(limit, fullSize)
	return &blobReader{
		rd:                r,
		n:                 limit,
		additionalDiscard: fullSize - limit,
		cancel:            cancel,
	}, fullSize, nil
}

type BlobTooLargeError struct {
	Size, Limit int64
}

func (b BlobTooLargeError) Error() string {
	return fmt.Sprintf("blob: content larger than limit (%d > %d)", b.Size, b.Limit)
}

// GetContentBase64 Reads the content of the blob and returns it as base64 encoded string.
// Returns [BlobTooLargeError] if the (unencoded) content is larger than the limit.
func (b *Blob) GetContentBase64(limit int64) (string, error) {
	rc, size, err := b.NewReader(limit)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	encoding := base64.StdEncoding
	buf := bytes.NewBuffer(make([]byte, 0, encoding.EncodedLen(int(size))))

	encoder := base64.NewEncoder(encoding, buf)

	if _, err := io.Copy(encoder, rc); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GuessContentType guesses the content type of the blob.
func (b *Blob) GuessContentType() (typesniffer.SniffedType, error) {
	r, err := b.DataAsync()
	if err != nil {
		return typesniffer.SniffedType{}, err
	}
	defer r.Close()

	return typesniffer.DetectContentTypeFromReader(r, b.Name())
}

// GetBlob finds the blob object in the repository.
func (repo *Repository) GetBlob(idStr string) (*Blob, error) {
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	if id.IsZero() {
		return nil, ErrNotExist{id.String(), ""}
	}
	return &Blob{
		ID:   id,
		repo: repo,
	}, nil
}
