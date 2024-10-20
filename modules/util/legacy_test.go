// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	testContent := []byte("hello")

	tmpDir := os.TempDir()
	now := time.Now()
	srcFile := fmt.Sprintf("%s/copy-test-%d-src.txt", tmpDir, now.UnixMicro())
	dstFile := fmt.Sprintf("%s/copy-test-%d-dst.txt", tmpDir, now.UnixMicro())

	_ = os.Remove(srcFile)
	_ = os.Remove(dstFile)
	defer func() {
		_ = os.Remove(srcFile)
		_ = os.Remove(dstFile)
	}()

	err := os.WriteFile(srcFile, testContent, 0o777)
	require.NoError(t, err)
	err = CopyFile(srcFile, dstFile)
	require.NoError(t, err)
	dstContent, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, dstContent)
}
