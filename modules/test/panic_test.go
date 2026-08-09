// Copyright 2026 The Forgejo Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanicError(t *testing.T) {
	message := "MESSAGE"
	err := errors.New(message)
	assert.True(t, PanicErrorContains(t, func() { panic(err) }, message))
	assert.True(t, PanicErrorIs(t, func() { panic(err) }, err))

	require.NotErrorIs(t, PanicToError(func() { panic(errors.New("OTHER")) }), err)
	require.NoError(t, PanicToError(func() {}))
}
