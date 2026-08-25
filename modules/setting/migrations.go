// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"time"

	"forgejo.org/modules/log"
)

// Migrations settings
var Migrations = struct {
	MaxAttempts        int
	RetryBackoff       int
	AllowedDomains     string
	BlockedDomains     string
	AllowLocalNetworks bool
	SkipTLSVerify      bool
	AllowUnencrypted   bool
	AvatarFetchTimeout time.Duration
}{
	MaxAttempts:  3,
	RetryBackoff: 3,
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	Migrations.AllowedDomains = sec.Key("ALLOWED_DOMAINS").MustString("")
	Migrations.BlockedDomains = sec.Key("BLOCKED_DOMAINS").MustString("")
	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)
	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
	Migrations.AllowUnencrypted = sec.Key("ALLOW_UNENCRYPTED").MustBool(false)

	var err error
	Migrations.AvatarFetchTimeout, err = sec.Key("AVATAR_FETCH_TIMEOUT").MustDuration(60 * time.Second)
	if err != nil {
		log.Fatal("Failed to parse duration for [migrations].AVATAR_FETCH_TIMEOUT: %w", err)
	}
}
