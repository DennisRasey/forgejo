// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package misc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultRobotsTxt(t *testing.T) {
	robotsTxt := string(defaultRobotsTxt())
	assert.Contains(t, robotsTxt, "https://forgejo.org/docs/latest/admin/advanced/search-engines/")
	assert.Equal(t, robotsTxt, string(defaultRobotsTxt()), "subsequent call still works")
}
