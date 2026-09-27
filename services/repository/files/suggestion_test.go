// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuggestionLines(t *testing.T) {
	assert.Nil(t, suggestionLines(""))                             // empty = delete the range
	assert.Equal(t, []string{"x"}, suggestionLines("x"))           // no trailing newline
	assert.Equal(t, []string{"x"}, suggestionLines("x\n"))         // synthetic trailing newline dropped
	assert.Equal(t, []string{"a", "b"}, suggestionLines("a\nb\n")) // multi-line
	assert.Equal(t, []string{"a", ""}, suggestionLines("a\n\n"))   // inner blank line kept
	assert.Equal(t, []string{"a", "", "b"}, suggestionLines("a\n\nb\n"))
}

func TestSpliceLines(t *testing.T) {
	t.Run("replace a middle line", func(t *testing.T) {
		got, err := spliceLines("a\nb\nc\n", 2, 2, []string{"B"})
		require.NoError(t, err)
		assert.Equal(t, "a\nB\nc\n", got)
	})

	t.Run("replace a multi-line range with fewer lines", func(t *testing.T) {
		got, err := spliceLines("a\nb\nc\nd\n", 2, 3, []string{"X"})
		require.NoError(t, err)
		assert.Equal(t, "a\nX\nd\n", got)
	})

	t.Run("empty replacement deletes the range", func(t *testing.T) {
		got, err := spliceLines("a\nb\nc\n", 2, 2, nil)
		require.NoError(t, err)
		assert.Equal(t, "a\nc\n", got)
	})

	t.Run("replace the last line keeps no-final-newline", func(t *testing.T) {
		got, err := spliceLines("a\nb", 2, 2, []string{"B"})
		require.NoError(t, err)
		assert.Equal(t, "a\nB", got)
	})

	t.Run("CRLF is preserved", func(t *testing.T) {
		got, err := spliceLines("a\r\nb\r\nc\r\n", 2, 2, []string{"B"})
		require.NoError(t, err)
		assert.Equal(t, "a\r\nB\r\nc\r\n", got)
	})

	t.Run("insert more lines than replaced", func(t *testing.T) {
		got, err := spliceLines("a\nb\n", 1, 1, []string{"x", "y"})
		require.NoError(t, err)
		assert.Equal(t, "x\ny\nb\n", got)
	})

	t.Run("out of bounds errors", func(t *testing.T) {
		_, err := spliceLines("a\nb\n", 0, 1, []string{"x"})
		require.Error(t, err)
		_, err = spliceLines("a\nb\n", 1, 9, []string{"x"})
		require.Error(t, err)
	})
}
