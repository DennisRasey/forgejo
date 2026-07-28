// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package card

import (
	"image/color"
	"testing"

	"github.com/golang/freetype/truetype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/font/gofont/goregular"
)

func TestNewCard(t *testing.T) {
	width, height := 100, 50
	card, err := NewCard(width, height)
	require.NoError(t, err, "No error should occur when creating a new card")
	assert.NotNil(t, card, "Card should not be nil")
	assert.Equal(t, width, card.Img.Bounds().Dx(), "Width should match the provided width")
	assert.Equal(t, height, card.Img.Bounds().Dy(), "Height should match the provided height")

	// Checking default margin
	assert.Equal(t, 0, card.Margin, "Default margin should be 0")

	// Checking font parsing
	originalFont, _ := truetype.Parse(goregular.TTF)
	assert.Equal(t, originalFont, card.Font, "Fonts should be equivalent")
}

func TestSplit(t *testing.T) {
	// Note: you normally wouldn't split the same card twice as draw operations would start to overlap each other; but
	// it's fine for this limited scope test
	card, _ := NewCard(200, 100)

	// Test vertical split
	leftCard, rightCard := card.Split(true, 50)
	assert.Equal(t, 100, leftCard.Img.Bounds().Dx(), "Left card should have half the width of original")
	assert.Equal(t, 100, leftCard.Img.Bounds().Dy(), "Left card height unchanged by split")
	assert.Equal(t, 100, rightCard.Img.Bounds().Dx(), "Right card should have half the width of original")
	assert.Equal(t, 100, rightCard.Img.Bounds().Dy(), "Right card height unchanged by split")

	// Test horizontal split
	topCard, bottomCard := card.Split(false, 50)
	assert.Equal(t, 200, topCard.Img.Bounds().Dx(), "Top card width unchanged by split")
	assert.Equal(t, 50, topCard.Img.Bounds().Dy(), "Top card should have half the height of original")
	assert.Equal(t, 200, bottomCard.Img.Bounds().Dx(), "Bottom width unchanged by split")
	assert.Equal(t, 50, bottomCard.Img.Bounds().Dy(), "Bottom card should have half the height of original")
}

func TestDrawTextSingleLine(t *testing.T) {
	card, _ := NewCard(300, 100)
	lines, err := card.DrawText("This is a single line", color.Black, 12, Middle, Center)
	require.NoError(t, err, "No error should occur when drawing text")
	assert.Len(t, lines, 1, "Should be exactly one line")
	assert.Equal(t, "This is a single line", lines[0], "Text should match the input")
}

func TestDrawTextLongLine(t *testing.T) {
	card, _ := NewCard(300, 100)
	text := "This text is definitely too long to fit in three hundred pixels width without wrapping"
	lines, err := card.DrawText(text, color.Black, 12, Middle, Center)
	require.NoError(t, err, "No error should occur when drawing text")
	assert.Len(t, lines, 2, "Text should wrap into multiple lines")
	assert.Equal(t, "This text is definitely too long to fit in three hundred", lines[0], "Text should match the input")
	assert.Equal(t, "pixels width without wrapping", lines[1], "Text should match the input")
}

func TestDrawTextWordTooLong(t *testing.T) {
	card, _ := NewCard(300, 100)
	text := "Line 1 Superduperlongwordthatcannotbewrappedbutshouldenduponitsownsingleline Line 3"
	lines, err := card.DrawText(text, color.Black, 12, Middle, Center)
	require.NoError(t, err, "No error should occur when drawing text")
	assert.Len(t, lines, 3, "Text should create two lines despite long word")
	assert.Equal(t, "Line 1", lines[0], "First line should contain text before the long word")
	assert.Equal(t, "Superduperlongwordthatcannotbewrappedbutshouldenduponitsownsingleline", lines[1], "Second line couldn't wrap the word so it just overflowed")
	assert.Equal(t, "Line 3", lines[2], "Third line continued with wrapping")
}
