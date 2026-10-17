// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgefed_test

import (
	"testing"

	"forgejo.org/modules/forgefed"
	"forgejo.org/modules/validation"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
)

func Test_UserActivityNoteValidation(t *testing.T) {
	sut := forgefed.ForgeUserActivityNote{}
	sut.Type = ap.NoteType
	sut.Content = ap.NaturalLanguageValues{
		ap.NilLangRef: ap.Content("Any Content!"),
	}
	sut.URL = ap.IRI("example.org/user-id/57")

	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)
}
