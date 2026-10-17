// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"testing"

	"forgejo.org/modules/validation"

	"github.com/stretchr/testify/require"
)

func Test_UserEmailValidate(t *testing.T) {
	sut := "ab@cd.ef"
	err := validation.ValidateEmail(sut)
	require.NoError(t, err, "'%s' should be valid", sut)

	sut = "83ce13c8-af0b-4112-8327-55a54e54e664@code.cartoon-aa.xyz"
	err = validation.ValidateEmail(sut)
	require.NoError(t, err, "'%s' should be valid", sut)

	sut = "1"
	err = validation.ValidateEmail(sut)
	require.Error(t, err, "'%s' should not be valid", sut)
}
