// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"forgejo.org/modules/validation"

	"github.com/stretchr/testify/assert"
)

func Test_FederatedUserValidation(t *testing.T) {
	sut := FederatedUser{
		UserID:           12,
		ExternalID:       "12",
		FederationHostID: 1,
		InboxPath:        "/api/v1/activitypub/user-id/12/inbox",
	}
	valid, err := validation.IsValid(sut)
	assert.True(t, valid, "sut '%v' should be valid: %v", sut, err)

	sut = FederatedUser{
		ExternalID:       "12",
		FederationHostID: 1,
		InboxPath:        "/api/v1/activitypub/user-id/12/inbox",
	}
	valid, _ = validation.IsValid(sut)
	assert.False(t, valid, "sut should be invalid")
}
