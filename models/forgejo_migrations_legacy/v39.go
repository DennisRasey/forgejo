// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations_legacy

import (
	"context"
	"fmt"

	"forgejo.org/models/db"
	secret_model "forgejo.org/models/secret"
	"forgejo.org/modules/log"
	"forgejo.org/modules/secret"
	"forgejo.org/modules/setting"

	"code.forgejo.org/xorm/xorm"
	"code.forgejo.org/xorm/xorm/schemas"
)

func MigrateActionSecretsToKeying(x *xorm.Engine) error {
	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)

		switch x.Dialect().URI().DBType {
		case schemas.MYSQL:
			if _, err := sess.Exec("ALTER TABLE `secret` MODIFY `data` BLOB"); err != nil {
				return err
			}
		case schemas.SQLITE:
			if _, err := sess.Exec("ALTER TABLE `secret` RENAME COLUMN `data` TO `data_backup`"); err != nil {
				return err
			}
			if _, err := sess.Exec("ALTER TABLE `secret` ADD COLUMN `data` BLOB"); err != nil {
				return err
			}
			if _, err := sess.Exec("UPDATE `secret` SET `data` = `data_backup`"); err != nil {
				return err
			}
			if _, err := sess.Exec("ALTER TABLE `secret` DROP COLUMN `data_backup`"); err != nil {
				return err
			}
		case schemas.POSTGRES:
			if _, err := sess.Exec("ALTER TABLE `secret` ALTER COLUMN `data` SET DATA TYPE bytea USING data::text::bytea"); err != nil {
				return err
			}
		}

		oldEncryptionKey := setting.SecretKey

		messages := make([]string, 0, 100)
		ids := make([]int64, 0, 100)

		err := db.Iterate(ctx, nil, func(ctx context.Context, bean *secret_model.Secret) error {
			secretBytes, err := secret.DecryptSecret(oldEncryptionKey, string(bean.Data))
			if err != nil {
				messages = append(messages, fmt.Sprintf("secret.id=%d, secret.name=%q, secret.repo_id=%d, secret.owner_id=%d: secret.DecryptSecret(): %v", bean.ID, bean.Name, bean.RepoID, bean.OwnerID, err))
				ids = append(ids, bean.ID)
				return nil
			}

			bean.SetData(secretBytes)
			_, err = sess.Cols("data").ID(bean.ID).Update(bean)
			return err
		})

		if err == nil {
			if len(ids) > 0 {
				log.Error("Forgejo migration[37]: The following action secrets were found to be corrupted and removed from the database.")
				for _, message := range messages {
					log.Error("Forgejo migration[37]: %s", message)
				}

				_, err = sess.In("id", ids).NoAutoCondition().NoAutoTime().Delete(&secret_model.Secret{})
			}
		}
		return err
	})
}
