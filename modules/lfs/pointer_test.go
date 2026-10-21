// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringContent(t *testing.T) {
	p := Pointer{Oid: "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393", Size: 1234}
	expected := "version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\n"
	assert.Equal(t, expected, p.StringContent())
}

func TestRelativePath(t *testing.T) {
	p := Pointer{Oid: "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393"}
	expected := path.Join("4d", "7a", "214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393")
	assert.Equal(t, expected, p.RelativePath())

	p2 := Pointer{Oid: "4d7a"}
	assert.Equal(t, "4d7a", p2.RelativePath())
}

func TestIsPointerValid(t *testing.T) {
	p := Pointer{}
	assert.Equal(t, ErrInvalidOIDFormat, p.Validate())

	p = Pointer{Oid: "123"}
	assert.False(t, p.IsOIDValid())
	assert.Equal(t, ErrInvalidOIDFormat, p.Validate())

	p = Pointer{Oid: "z4cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc"}
	assert.False(t, p.IsOIDValid())
	assert.Equal(t, ErrInvalidOIDFormat, p.Validate())

	p = Pointer{Oid: "94cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc"}
	assert.True(t, p.IsOIDValid())
	require.NoError(t, p.Validate())

	p = Pointer{Oid: "94cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc", Size: -1}
	assert.True(t, p.IsOIDValid())
	assert.Equal(t, ErrInvalidPointerTargetSize, p.Validate())

	p = Pointer{Oid: "94cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc", Size: 0}
	assert.True(t, p.IsOIDValid())
	require.NoError(t, p.Validate())

	p = Pointer{Oid: "94cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc", Size: 1}
	assert.True(t, p.IsOIDValid())
	require.NoError(t, p.Validate())

	p = Pointer{Oid: "αυτή η πρόταση παραβιάζει τους περιορισμούς του regex", Size: 1}
	assert.False(t, p.IsOIDValid())
	assert.Equal(t, ErrInvalidOIDFormat, p.Validate())
}

func TestGeneratePointer(t *testing.T) {
	p, err := GeneratePointer(strings.NewReader("Forgejo"))
	require.NoError(t, err)
	require.NoError(t, p.Validate())
	assert.Equal(t, "f4e62146dfbeb0b41240f464a02a9d1ec232eeab631a998365ac04b99f1884e6", p.Oid)
	assert.Equal(t, int64(7), p.Size)
}

func TestReadPointerFromBuffer(t *testing.T) {
	// expectedErr == nil -> any error
	simpleTestBufferAndStringReadError := func(testString string, expectedErr error) {
		_, err := ReadPointerFromString(testString)
		if expectedErr != nil {
			require.ErrorIs(t, err, expectedErr)
		} else {
			require.Error(t, err)
		}
		_, err = ReadPointerFromBuffer([]byte(testString))
		if expectedErr != nil {
			require.ErrorIs(t, err, expectedErr)
		} else {
			require.Error(t, err)
		}
	}

	simpleTestBufferAndStringReadError("", ErrMissingPrefix)
	simpleTestBufferAndStringReadError("test", ErrMissingPrefix)
	simpleTestBufferAndStringReadError("version https://git-lfs.github.com/spec/v1\n", ErrInvalidStructure)
	simpleTestBufferAndStringReadError("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a\nsize 1234\n", ErrInvalidOIDFormat)
	simpleTestBufferAndStringReadError("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a2146z4ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\n", ErrInvalidOIDFormat)
	simpleTestBufferAndStringReadError("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize -1\n", ErrInvalidPointerTargetSize)
	simpleTestBufferAndStringReadError("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\ntest 1234\n", nil)
	simpleTestBufferAndStringReadError("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize test\n", nil)

	p, err := ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\ntest"))
	require.NoError(t, err)
	assert.NoError(t, p.Validate())
	assert.Equal(t, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393", p.Oid)
	assert.Equal(t, int64(1234), p.Size)

	p, err = ReadPointerFromString("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\ntest")
	require.NoError(t, err)
	assert.NoError(t, p.Validate())
	assert.Equal(t, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393", p.Oid)
	assert.Equal(t, int64(1234), p.Size)
}

func TestReadPointer(t *testing.T) {
	p, err := ReadPointer(strings.NewReader("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\n"))
	require.NoError(t, err)
	assert.NoError(t, p.Validate())
	assert.Equal(t, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393", p.Oid)
	assert.Equal(t, int64(1234), p.Size)
}
