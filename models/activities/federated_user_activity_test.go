// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"testing"

	"forgejo.org/modules/validation"

	"github.com/stretchr/testify/assert"
)

func Test_FederatedUserActivityValidation(t *testing.T) {
	sut := FederatedUserActivity{}
	sut.UserID = 13
	sut.ActorID = 33
	sut.ActorURI = "33"
	sut.NoteContent = "Any content!"
	sut.NoteURL = "https://example.org/note/17"
	sut.OriginalNote = "federatedUserActivityNote-17"

	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)
}
