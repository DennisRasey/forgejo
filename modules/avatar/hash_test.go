// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar_test

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"forgejo.org/modules/avatar"

	"github.com/stretchr/testify/assert"
)

func Test_HashAvatar(t *testing.T) {
	myImage := image.NewRGBA(image.Rect(0, 0, 32, 32))
	var buff bytes.Buffer
	png.Encode(&buff, myImage)

	assert.Equal(t, "1481b91e790a4875edb9157cca7a2a9533dcd9ece9f180a0a2956540961dda1a", avatar.HashAvatar(1, buff.Bytes()))
	assert.Equal(t, "5c8852abb7496e228de352b7aeac976c645bcb204ef015bb53bbd741c5c7f0c3", avatar.HashAvatar(8, buff.Bytes()))
	assert.Equal(t, "fe711fc79163c873c9980d5dd44240ad9e15565e31052eeb3cb3dfe515902975", avatar.HashAvatar(1024, buff.Bytes()))
	assert.Equal(t, "161178642c7d59eb25a61dddced5e6b66eae1c70880d5f148b1b497b767e72d9", avatar.HashAvatar(1024, []byte{}))
}
