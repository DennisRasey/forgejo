// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
	"forgejo.org/modules/json"
	"forgejo.org/modules/optional"

	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

func init() {
	registerMigration(&Migration{
		Description: "add OIDCSubjectFormat=legacy-forgejo-v15 to all existing repositories with actions enabled",
		Upgrade:     setOIDCSubjectFormatLegacy15,
	})
}

func setOIDCSubjectFormatLegacy15(x *xorm.Engine) error {
	type Type int
	const TypeActions Type = 10 // 10 Actions
	type ActionsConfig struct {
		DisabledWorkflows []string `json:",omitempty"`
		OIDCSubjectFormat string
	}
	type RepoUnit struct { //revive:disable-line:exported
		ID     int64                   `xorm:"pk"`
		Type   Type                    `xorm:"INDEX(s)"`
		Config optional.Option[string] `xorm:"TEXT"`
	}

	return db.Iterate(
		db.DefaultContext,
		builder.Eq{"type": TypeActions},
		func(ctx context.Context, unit *RepoUnit) error {
			has, config := unit.Config.Get()
			if !has {
				config = "{}"
			}
			var actionsConfig ActionsConfig
			if err := json.Unmarshal([]byte(config), &actionsConfig); err != nil {
				return fmt.Errorf("failed to parse Actions config %q: %w", config, err)
			}

			actionsConfig.OIDCSubjectFormat = "legacy-forgejo-v15"

			configBytes, err := json.Marshal(actionsConfig)
			if err != nil {
				return fmt.Errorf("failed to convert Actions config to JSON: %w", err)
			}
			r, err := db.GetEngine(ctx).
				ID(unit.ID).
				Cols("config").
				Update(&RepoUnit{Config: optional.Some(string(configBytes))})
			if err != nil {
				return err
			} else if r != 1 {
				return fmt.Errorf("UPDATE expected to affect 1 row, but was %d", r)
			}
			return nil
		},
	)
}
