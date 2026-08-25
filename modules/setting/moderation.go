// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package setting

import (
	"fmt"
	"time"
)

// Moderation settings
var Moderation = struct {
	Enabled                bool          `ini:"ENABLED"`
	KeepResolvedReportsFor time.Duration `ini:"KEEP_RESOLVED_REPORTS_FOR"`
}{
	Enabled: false,
}

func loadModerationFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("moderation")
	err := sec.MapTo(&Moderation)
	if err != nil {
		return fmt.Errorf("failed to map Moderation settings: %v", err)
	}

	// keep reports for one week by default. Since time.Duration stops at the unit of an hour
	// we are using the value of 24 (hours) * 7 (days) which gives us the value of 168
	Moderation.KeepResolvedReportsFor, err = sec.Key("KEEP_RESOLVED_REPORTS_FOR").MustDuration(168 * time.Hour)
	if err != nil {
		return fmt.Errorf("failed to parse duration for [moderation].KEEP_RESOLVED_REPORTS_FOR: %w", err)
	}
	return nil
}
