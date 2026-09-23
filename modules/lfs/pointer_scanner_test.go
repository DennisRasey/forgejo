// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package lfs

import (
	"cmp"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"forgejo.org/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchPointerBlobs(t *testing.T) {
	repo, err := git.OpenRepository(t.Context(), filepath.Join(testReposDir, "simple-lfs"))
	require.NoError(t, err)

	pointerChan := make(chan PointerBlob)
	errChan := make(chan error, 1)

	go SearchPointerBlobs(t.Context(), repo, pointerChan, errChan)

	// Collect all found pointer blobs.
	var pointerBlobs []PointerBlob
	for pointerBlob := range pointerChan {
		pointerBlobs = append(pointerBlobs, pointerBlob)
	}

	// Check that no errors were reported.
	select {
	case err, ok := <-errChan:
		require.NoError(t, err)
		assert.False(t, ok, "errChan should have been closed")
	case <-time.After(10 * time.Second):
		t.Fatal("SearchPointerBlobs took too long to return")
	}

	// Sort them, they might arrive in any order
	slices.SortFunc(pointerBlobs, func(a, b PointerBlob) int {
		return cmp.Compare(a.Oid, b.Oid)
	})

	// Assert the values of the found pointer blobs.
	if assert.Len(t, pointerBlobs, 3) {
		assert.Equal(t, "31b9a6a709729b8ae48bde8176caf2990c0d7121", pointerBlobs[0].Hash)
		assert.Equal(t, "2f91b6326743db344ca96b1be86e3ed34abf04262255b4d04db8a961a2a72545", pointerBlobs[0].Oid)
		assert.EqualValues(t, 43789, pointerBlobs[0].Size)

		assert.Equal(t, "b53619fbbc3d2bfa85a26787238264cdbf551f19", pointerBlobs[1].Hash)
		assert.Equal(t, "36dae031efb96625cda973c11508617b750665933a36bd52dfcfef586c4fd85c", pointerBlobs[1].Oid)
		assert.EqualValues(t, 101800, pointerBlobs[1].Size)

		assert.Equal(t, "513d4c000f63ee9fd6a805e9a518206b860ce38a", pointerBlobs[2].Hash)
		assert.Equal(t, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393", pointerBlobs[2].Oid)
		assert.EqualValues(t, 1234, pointerBlobs[2].Size)
	}
}
