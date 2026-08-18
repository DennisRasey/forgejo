// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"forgejo.org/models/dbfs"
	"forgejo.org/modules/log"
	"forgejo.org/modules/storage"
	"forgejo.org/modules/zstd"

	runnerv1 "code.forgejo.org/forgejo/actions-proto/runner/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	MaxLineSize = 64 * 1024
	DBFSPrefix  = "actions_log/"

	timeFormat     = "2006-01-02T15:04:05.0000000Z07:00"
	defaultBufSize = MaxLineSize

	// MaxStoredLineSize is the upper bound on a single stored log line as it
	// appears on disk: the timestamp prefix, then a space, then up to
	// MaxLineSize bytes of content. Callers using bufio.Scanner over stored
	// log bytes should size their buffer at least this large.
	MaxStoredLineSize = len(timeFormat) + MaxLineSize + 1
)

func ExistsLogs(ctx context.Context, filename string) (bool, error) {
	name := DBFSPrefix + filename
	f, err := dbfs.Open(ctx, name)
	if err == nil {
		f.Close()
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// WriteLogs appends logs to DBFS file for temporary storage.
// It doesn't respect the file format in the filename like ".zst", since it's difficult to reopen a closed compressed file and append new content.
// Why doesn't it store logs in object storage directly? Because it's not efficient to append content to object storage.
func WriteLogs(ctx context.Context, filename string, offset int64, rows []*runnerv1.LogRow) ([]int, error) {
	flag := os.O_WRONLY
	if offset == 0 {
		// Create file only if offset is 0, or it could result in content holes if the file doesn't exist.
		flag |= os.O_CREATE
	}
	name := DBFSPrefix + filename
	f, err := dbfs.OpenFile(ctx, name, flag)
	if err != nil {
		return nil, fmt.Errorf("dbfs OpenFile %q: %w", name, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("WriteLogs(name=%q, offset=%d, len(rows)=%d): dbfs Stat: %w", name, offset, len(rows), err)
	}
	if stat.Size() < offset {
		// If the size is less than offset, refuse to write, or it could result in content holes.
		// However, if the size is greater than offset, we can still write to overwrite the content.
		return nil, fmt.Errorf("size of %q is less than offset", name)
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("dbfs Seek %q: %w", name, err)
	}

	writer := bufio.NewWriterSize(f, defaultBufSize)

	ns := make([]int, 0, len(rows))
	for _, row := range rows {
		n, err := writer.WriteString(FormatLog(row.Time.AsTime(), row.Content) + "\n")
		if err != nil {
			return nil, err
		}
		ns = append(ns, n)
	}

	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return ns, nil
}

func ReadLogs(ctx context.Context, inStorage bool, filename string, offset, limit int64) ([]*runnerv1.LogRow, error) {
	f, err := OpenLogs(ctx, inStorage, filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("file seek: %w", err)
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, MaxStoredLineSize), MaxStoredLineSize)

	var rows []*runnerv1.LogRow
	for scanner.Scan() && (int64(len(rows)) < limit || limit < 0) {
		t, c, err := ParseLog(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("parse log %q: %w", scanner.Text(), err)
		}
		rows = append(rows, &runnerv1.LogRow{
			Time:    timestamppb.New(t),
			Content: c,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ReadLogs scan: %w", err)
	}

	return rows, nil
}

const (
	// logZstdBlockSize is the block size for zstd compression.
	// 128KB leads the compression ratio to be close to the regular zstd compression.
	// And it means each read from the underlying object storage will be at least 128KB*(compression ratio).
	// The compression ratio is about 30% for text files, so the actual read size is about 38KB, which should be acceptable.
	logZstdBlockSize = 128 * 1024 // 128KB
)

// TransferLogs transfers logs from DBFS to object storage.
// It happens when the file is complete and no more logs will be appended.
// It respects the file format in the filename like ".zst", and compresses the content if needed.
func TransferLogs(ctx context.Context, filename string) (func(), error) {
	name := DBFSPrefix + filename
	remove := func() {
		if err := dbfs.Remove(ctx, name); err != nil {
			log.Warn("dbfs remove %q: %v", name, err)
		}
	}
	f, err := dbfs.Open(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("dbfs open %q: %w", name, err)
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(filename, ".zst") {
		r, w := io.Pipe()
		reader = r
		zstdWriter, err := zstd.NewSeekableWriter(w, logZstdBlockSize)
		if err != nil {
			return nil, fmt.Errorf("zstd NewSeekableWriter: %w", err)
		}
		go func() {
			defer func() {
				_ = w.CloseWithError(zstdWriter.Close())
			}()
			if _, err := io.Copy(zstdWriter, f); err != nil {
				_ = w.CloseWithError(err)
				return
			}
		}()
	}

	if _, err := storage.Actions.Save(filename, reader, -1); err != nil {
		return nil, fmt.Errorf("storage save %q: %w", filename, err)
	}
	return remove, nil
}

// RemoveLogs deletes the log file with the given filename. Returns an error if filename is empty.
func RemoveLogs(ctx context.Context, inStorage bool, filename string) error {
	if filename == "" {
		return errors.New("cannot remove logs because filename is empty")
	}

	if !inStorage {
		name := DBFSPrefix + filename
		err := dbfs.Remove(ctx, name)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("dbfs remove %q: %w", name, err)
		}
		return nil
	}
	err := storage.Actions.Delete(filename)
	if err != nil {
		return fmt.Errorf("storage delete %q: %w", filename, err)
	}
	return nil
}

func OpenLogs(ctx context.Context, inStorage bool, filename string) (io.ReadSeekCloser, error) {
	if !inStorage {
		name := DBFSPrefix + filename
		f, err := dbfs.Open(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("dbfs open %q: %w", name, err)
		}
		return f, nil
	}

	f, err := storage.Actions.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("storage open %q: %w", filename, err)
	}

	var reader io.ReadSeekCloser = f
	if strings.HasSuffix(filename, ".zst") {
		r, err := zstd.NewSeekableReader(f)
		if err != nil {
			return nil, fmt.Errorf("zstd NewSeekableReader: %w", err)
		}
		reader = r
	}

	return reader, nil
}

// OpenLogsRange opens the log at filename and returns an io.ReadSeekCloser
// exposing exactly [offset, offset+size) bytes as if it started at offset 0.
// Byte-window sibling of ReadLogs: use this when a seekable byte stream is
// needed (e.g. http.ServeContent Range support) rather than parsed line rows.
func OpenLogsRange(ctx context.Context, inStorage bool, filename string, offset, size int64) (io.ReadSeekCloser, error) {
	reader, err := OpenLogs(ctx, inStorage, filename)
	if err != nil {
		return nil, err
	}
	bounded, err := newBoundedReadSeekCloser(reader, offset, size)
	if err != nil {
		reader.Close()
		return nil, fmt.Errorf("could not position log reader at offset %d: %w", offset, err)
	}
	return bounded, nil
}

// boundedReadSeekCloser exposes a [start, start+size) window of the underlying
// reader as if it started at offset 0. Backs OpenLogsRange so http.ServeContent
// can serve Range requests over a step-sized slice without buffering.
type boundedReadSeekCloser struct {
	inner io.ReadSeekCloser
	start int64
	size  int64
	pos   int64
}

func newBoundedReadSeekCloser(inner io.ReadSeekCloser, start, size int64) (*boundedReadSeekCloser, error) {
	if _, err := inner.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return &boundedReadSeekCloser{inner: inner, start: start, size: size}, nil
}

func (b *boundedReadSeekCloser) Read(p []byte) (int, error) {
	if b.pos >= b.size {
		return 0, io.EOF
	}
	if remaining := b.size - b.pos; int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := b.inner.Read(p)
	b.pos += int64(n)
	return n, err
}

func (b *boundedReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = b.pos + offset
	case io.SeekEnd:
		abs = b.size + offset
	default:
		return 0, errors.New("boundedReadSeekCloser: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("boundedReadSeekCloser: negative position")
	}
	if _, err := b.inner.Seek(b.start+abs, io.SeekStart); err != nil {
		return 0, err
	}
	b.pos = abs
	return abs, nil
}

func (b *boundedReadSeekCloser) Close() error { return b.inner.Close() }

func FormatLog(timestamp time.Time, content string) string {
	// Content shouldn't contain new line, it will break log indexes, other control chars are safe.
	content = strings.ReplaceAll(content, "\n", `\n`)
	if len(content) > MaxLineSize {
		content = content[:MaxLineSize]
	}
	return fmt.Sprintf("%s %s", timestamp.UTC().Format(timeFormat), content)
}

func ParseLog(in string) (time.Time, string, error) {
	index := strings.IndexRune(in, ' ')
	if index < 0 {
		return time.Time{}, "", fmt.Errorf("invalid log: %q", in)
	}
	timestamp, err := time.Parse(timeFormat, in[:index])
	if err != nil {
		return time.Time{}, "", err
	}
	return timestamp, in[index+1:], nil
}
