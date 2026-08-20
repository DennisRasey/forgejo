// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Project settings
var (
	Project = struct {
		ProjectBoardBasicKanbanType []string
		ProjectBoardBugTriageType   []string
	}{
		ProjectBoardBasicKanbanType: []string{"To do", "In progress", "Done"},
		ProjectBoardBugTriageType:   []string{"Needs triage", "High priority", "Low priority", "Closed"},
	}
)

func loadProjectFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "project", &Project)
}
