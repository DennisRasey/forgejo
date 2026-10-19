// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"net/http"
	"testing"

	"forgejo.org/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func Test2FARedirect(t *testing.T) {
	ctx, resp := contexttest.MockContext(t, "/user/two_factor")
	TwoFactor(ctx)
	assert.Equal(t, http.StatusFound, resp.Code)
	assert.Equal(t, "/user/login", resp.Header().Get("location"))
}
