// Copyright 2026 The Forgejo Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func PanicErrorContains(t *testing.T, fun func(), contains string) bool {
	t.Helper()
	return assert.ErrorContains(t, PanicToError(fun), contains)
}

func PanicErrorIs(t *testing.T, fun func(), is error) bool {
	t.Helper()
	return assert.ErrorIs(t, PanicToError(fun), is)
}

func PanicToError(fun func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			recoveredErr, ok := r.(error)
			if !ok {
				recoveredErr = fmt.Errorf("%v", r)
			}
			err = recoveredErr
		}
	}()

	fun()

	return err
}
