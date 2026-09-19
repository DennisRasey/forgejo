// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

type (
	// TemplateType is used to represent a project template type
	TemplateType uint8

	// TemplateConfig is used to identify the template type of project that is being created
	TemplateConfig struct {
		TemplateType TemplateType
		Translation  string
	}
)

const (
	// TemplateTypeNone is a project template type that has no predefined columns
	TemplateTypeNone TemplateType = iota

	// TemplateTypeBasicKanban is a project template type that has basic predefined columns
	TemplateTypeBasicKanban

	// TemplateTypeBugTriage is a project template type that has predefined columns suited to hunting down bugs
	TemplateTypeBugTriage
)
