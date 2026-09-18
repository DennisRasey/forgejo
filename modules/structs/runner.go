// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package structs

// RegisterRunnerOptions declares the accepted options for registering runners.
// swagger:model
type RegisterRunnerOptions struct {
	// Name of the runner to register. The name of the runner does not have to be unique.
	//
	// required: true
	Name string `json:"name" binding:"Required"`

	// Description of the runner to register.
	//
	// required: false
	Description string `json:"description"`

	// Register as ephemeral runner https://forgejo.org/docs/latest/admin/actions/security/#ephemeral-runners
	//
	// required: false
	Ephemeral bool `json:"ephemeral"`
}

// RegisterRunnerResponse contains the details of the just registered runner.
// swagger:model
type RegisterRunnerResponse struct {
	ID    int64  `json:"id" binding:"Required"`
	UUID  string `json:"uuid" binding:"Required"`
	Token string `json:"token" binding:"Required"`
}
