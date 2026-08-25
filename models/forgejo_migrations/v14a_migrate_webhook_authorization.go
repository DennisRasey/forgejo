// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
	"forgejo.org/modules/keying"
	"forgejo.org/modules/log"
	"forgejo.org/modules/secret"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
	"code.forgejo.org/xorm/xorm/schemas"
)

func init() {
	registerMigration(&Migration{
		Description: "migrate `header_authorization_encrypted` of `webhook` table to store keying material",
		Upgrade:     migrateWebhookSecrets,
	})
}

func migrateWebhookSecrets(x *xorm.Engine) error {
	type Webhook struct {
		ID                           int64  `xorm:"pk autoincr"`
		RepoID                       int64  `xorm:"INDEX"` // An ID of 0 indicates either a default or system webhook
		OwnerID                      int64  `xorm:"INDEX"`
		HeaderAuthorizationEncrypted []byte `xorm:"BLOB"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)

		switch x.Dialect().URI().DBType {
		case schemas.MYSQL:
			if _, err := sess.Exec("ALTER TABLE `webhook` MODIFY `header_authorization_encrypted` BLOB"); err != nil {
				return err
			}
		case schemas.SQLITE:
			if _, err := sess.Exec("ALTER TABLE `webhook` RENAME COLUMN `header_authorization_encrypted` TO `header_authorization_encrypted_backup`"); err != nil {
				return err
			}
			if _, err := sess.Exec("ALTER TABLE `webhook` ADD COLUMN `header_authorization_encrypted` BLOB"); err != nil {
				return err
			}
			if _, err := sess.Exec("UPDATE `webhook` SET `header_authorization_encrypted` = `header_authorization_encrypted_backup`"); err != nil {
				return err
			}
			if _, err := sess.Exec("ALTER TABLE `webhook` DROP COLUMN `header_authorization_encrypted_backup`"); err != nil {
				return err
			}
		case schemas.POSTGRES:
			if _, err := sess.Exec("ALTER TABLE `webhook` ALTER COLUMN `header_authorization_encrypted` SET DATA TYPE bytea USING header_authorization_encrypted::text::bytea"); err != nil {
				return err
			}
		}

		key := keying.Webhook

		oldEncryptionKey := setting.SecretKey
		messages := make([]string, 0, 100)
		ids := make([]int64, 0, 100)

		err := db.Iterate(ctx, nil, func(ctx context.Context, bean *Webhook) error {
			if len(bean.HeaderAuthorizationEncrypted) == 0 {
				return nil
			}

			secretBytes, err := secret.DecryptSecret(oldEncryptionKey, string(bean.HeaderAuthorizationEncrypted))
			if err != nil {
				messages = append(messages, fmt.Sprintf("webhook.id=%d, webhook.repo_id=%d, webhook.owner_id=%d: secret.DecryptSecret(): %v", bean.ID, bean.RepoID, bean.OwnerID, err))
				ids = append(ids, bean.ID)
				return nil
			}

			bean.HeaderAuthorizationEncrypted = key.Encrypt([]byte(secretBytes), keying.ColumnAndID("header_authorization_encrypted", bean.ID))
			_, err = sess.Cols("header_authorization_encrypted").ID(bean.ID).Update(bean)
			return err
		})

		if err == nil {
			if len(ids) > 0 {
				log.Error("migration[v14a_migrate_webhook_authorization]: The following webhook were found to be corrupted and removed from the database.")
				for _, message := range messages {
					log.Error("migration[v14a_migrate_webhook_authorization]: %s", message)
				}

				_, err = sess.In("id", ids).NoAutoCondition().NoAutoTime().Delete(&Webhook{})
			}
		}
		return err
	})
}
