// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaultProjectFrom(t *testing.T) {
	iniStr := ``
	cfg, err := NewConfigProviderFromData(iniStr)
	require.NoError(t, err)

	// Load project settings from empty config
	loadProjectFrom(cfg)

	// Verify default values are set
	assert.Equal(t, []string{"To do", "In progress", "Done"}, Project.ProjectBoardBasicKanbanType)
	assert.Equal(t, []string{"Needs triage", "High priority", "Low priority", "Closed"}, Project.ProjectBoardBugTriageType)
}

func TestLoadProjectFromOverrideConfig(t *testing.T) {
	iniStr := `
[project]
PROJECT_BOARD_BASIC_KANBAN_TYPE = Backlog,In Progress,Review,Done
PROJECT_BOARD_BUG_TRIAGE_TYPE = Unreviewed,Reviewed,Fixed
`
	cfg, err := NewConfigProviderFromData(iniStr)
	require.NoError(t, err)

	// Load project settings from config
	loadProjectFrom(cfg)

	// Verify custom values are loaded
	assert.Equal(t, []string{"Backlog", "In Progress", "Review", "Done"}, Project.ProjectBoardBasicKanbanType)
	assert.Equal(t, []string{"Unreviewed", "Reviewed", "Fixed"}, Project.ProjectBoardBugTriageType)
}
