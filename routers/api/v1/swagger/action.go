// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "forgejo.org/modules/structs"
	shared "forgejo.org/routers/api/v1/shared"
)

// SecretList
// swagger:response SecretList
type swaggerResponseSecretList struct {
	// in:body
	Body []api.Secret `json:"body"`

	// The total number of secrets
	TotalCount int64 `json:"X-Total-Count"`
}

// Secret
// swagger:response Secret
type swaggerResponseSecret struct {
	// in:body
	Body api.Secret `json:"body"`
}

// ActionVariable
// swagger:response ActionVariable
type swaggerResponseActionVariable struct {
	// in:body
	Body api.ActionVariable `json:"body"`
}

// VariableList
// swagger:response VariableList
type swaggerResponseVariableList struct {
	// in:body
	Body []api.ActionVariable `json:"body"`

	// The total number of variables
	TotalCount int64 `json:"X-Total-Count"`
}

// RunJobList is a list of action run jobs
// swagger:response RunJobList
type swaggerRunJobList struct {
	// in:body
	Body []*api.ActionRunJob `json:"body"`
}

// ActionRunJob is a single workflow run job
// swagger:response ActionRunJob
type swaggerActionRunJob struct {
	// in:body
	Body *api.ActionRunJob `json:"body"`
}

// DispatchWorkflowRun is a Workflow Run after dispatching
// swagger:response DispatchWorkflowRun
type swaggerDispatchWorkflowRun struct {
	// in:body
	Body *api.DispatchWorkflowRun `json:"body"`
}

// RegistrationToken is a string used to register a runner with a server
// swagger:response RegistrationToken
type swaggerRegistrationToken struct {
	// in: body
	Body shared.RegistrationToken `json:"body"`
}

// ActionRunner represents a runner
// swagger:response ActionRunner
type swaggerActionRunner struct {
	// in: body
	Body api.ActionRunner `json:"body"`
}

// ActionRunnerList is a list of Forgejo Action runners
// swagger:response ActionRunnerList
type swaggerActionRunnerListResponse struct {
	// in:body
	Body []api.ActionRunner `json:"body"`

	// Total number of runners matching the search criteria (excluding page and limit)
	TotalCount int64 `json:"X-Total-Count"`

	// Links to other pages, if any
	Link string `json:"Link"`
}

// RegisterRunnerResponse contains the details of the just registered runner.
// swagger:response RegisterRunnerResponse
type swaggerRegisterRunnerResponse struct {
	// in: body
	Body api.RegisterRunnerResponse `json:"body"`
}
