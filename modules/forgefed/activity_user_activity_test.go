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

func Test_ForgeUserActivityValidation(t *testing.T) {
	note := forgefed.ForgeUserActivityNote{}
	note.Type = ap.NoteType
	note.Content = ap.NaturalLanguageValues{
		ap.NilLangRef: ap.Content("Any Content!"),
	}
	note.URL = ap.IRI("example.org/user-id/57")

	sut := forgefed.ForgeUserActivity{}
	sut.Type = ap.CreateType
	sut.Actor = ap.IRI("example.org/user-id/23")
	sut.CC = ap.ItemCollection{
		ap.IRI("example.org/registration/public#2nd"),
	}
	sut.To = ap.ItemCollection{
		ap.IRI("example.org/registration/public"),
	}

	sut.Note = note

	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)
}
