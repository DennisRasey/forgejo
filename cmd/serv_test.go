// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"testing"

	asymkey_model "forgejo.org/models/asymkey"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/log"
)

func TestGetKeyCheckMessage(t *testing.T) {
	for _, tt := range []struct {
		name        string
		key         *asymkey_model.PublicKey
		user        *user_model.User
		appName     string
		expectedMsg string
	}{
		{
			name: "deploy key",
			key: &asymkey_model.PublicKey{
				Type:    asymkey_model.KeyTypeDeploy,
				Name:    "deploy-test-key",
				Content: "key contents",
			},
			user: &user_model.User{
				Name: "test-user",
			},
			appName: "Codeberg",
			expectedMsg: "Hi there! You've successfully authenticated with the deploy key named " +
				"deploy-test-key, but Codeberg does not provide shell access.\nIf this is " +
				"unexpected, please log in with password and setup Codeberg under another user.",
		},
		{
			name: "principal",
			key: &asymkey_model.PublicKey{
				Type:    asymkey_model.KeyTypePrincipal,
				Name:    "principal-test-key",
				Content: "key contents",
			},
			user: &user_model.User{
				Name: "test-user",
			},
			appName: "Forgejo: Beyond coding. We forge.",
			expectedMsg: "Hi there! You've successfully authenticated with the principal " +
				"key contents, but this Forgejo instance does not provide shell access.\n" +
				"If this is unexpected, please log in with password and setup this Forgejo " +
				"instance under another user.",
		},
		{
			name: "user",
			key: &asymkey_model.PublicKey{
				Type:    asymkey_model.KeyTypeUser,
				Name:    "user-test-key",
				Content: "key contents",
			},
			user: &user_model.User{
				Name: "test-user",
			},
			appName: "Social Coding",
			expectedMsg: "Hi there, test-user! You've successfully authenticated with the key " +
				"named user-test-key, but Social Coding does not provide shell access.\n" +
				"If this is unexpected, please log in with password and setup Social Coding " +
				"under another user.",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg := getKeyCheckMessage(tt.key, tt.user, tt.appName)
			if gotMsg != tt.expectedMsg {
				log.Fatal("expected key success message '%s' but got '%s'.", tt.expectedMsg, gotMsg)
			}
		})
	}
}
