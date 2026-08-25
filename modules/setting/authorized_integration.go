// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package setting

import (
	"time"

	"forgejo.org/modules/log"
)

var AuthorizedIntegration = struct {
	AllowedDomains     string
	BlockedDomains     string
	AllowLocalNetworks bool
	RequestTimeout     time.Duration
	CacheTTL           time.Duration
}{}

func loadAuthorizedIntegrationFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("authorized_integration")
	AuthorizedIntegration.AllowedDomains = sec.Key("ALLOWED_DOMAINS").MustString("")
	AuthorizedIntegration.BlockedDomains = sec.Key("BLOCKED_DOMAINS").MustString("")
	AuthorizedIntegration.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)

	var err error
	AuthorizedIntegration.RequestTimeout, err = sec.Key("REQUEST_TIMEOUT").MustDuration(10 * time.Second)
	if err != nil {
		log.Fatal("Failed to parse duration for [authorized_integration].REQUEST_TIMEOUT: %v", err)
	}
	AuthorizedIntegration.CacheTTL, err = sec.Key("CACHE_TTL").MustDuration(10 * time.Minute)
	if err != nil {
		log.Fatal("Failed to parse duration for [authorized_integration].CACHE_TTL: %v", err)
	}
}
