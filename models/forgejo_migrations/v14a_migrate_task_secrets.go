// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo_migrations

import (
	"context"
	"encoding/base64"
	"fmt"

	"forgejo.org/models/db"
	"forgejo.org/modules/json"
	"forgejo.org/modules/keying"
	"forgejo.org/modules/log"
	"forgejo.org/modules/migration"
	"forgejo.org/modules/secret"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/timeutil"

	"code.forgejo.org/xorm/xorm"
	"xorm.io/builder"
)

func init() {
	registerMigration(&Migration{
		Description: "migrate columns of `task` table to store keying material",
		Upgrade:     migrateTaskSecrets,
	})
}

func migrateTaskSecrets(x *xorm.Engine) error {
	type Task struct {
		ID             int64
		DoerID         int64              `xorm:"index"`
		OwnerID        int64              `xorm:"index"`
		RepoID         int64              `xorm:"index"`
		PayloadContent string             `xorm:"TEXT"`
		Created        timeutil.TimeStamp `xorm:"created"`
	}
	taskUpdateCols := func(ctx context.Context, task *Task, cols ...string) error {
		_, err := db.GetEngine(ctx).ID(task.ID).Cols(cols...).Update(task)
		return err
	}

	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)

		key := keying.MigrateTask

		oldEncryptionKey := setting.SecretKey
		messages := make([]string, 0, 100)
		ids := make([]int64, 0, 100)

		err := db.Iterate(ctx, builder.Eq{"type": structs.TaskTypeMigrateRepo}, func(ctx context.Context, bean *Task) error {
			var opts migration.MigrateOptions
			err := json.Unmarshal([]byte(bean.PayloadContent), &opts)
			if err != nil {
				messages = append(messages, fmt.Sprintf("task.id=%d, task.doer_id=%d, task.repo_id=%d, task.owner_id=%d: json.Unmarshal(): %v", bean.ID, bean.DoerID, bean.RepoID, bean.OwnerID, err))
				ids = append(ids, bean.ID)
				return nil
			}

			decryptionError := false
			if opts.CloneAddrEncrypted != "" {
				if opts.CloneAddr, err = secret.DecryptSecret(oldEncryptionKey, opts.CloneAddrEncrypted); err != nil {
					messages = append(messages, fmt.Sprintf("task.id=%d, task.doer_id=%d, task.repo_id=%d, task.owner_id=%d: secret.DecryptSecret(CloneAddrEncrypted): %v", bean.ID, bean.DoerID, bean.RepoID, bean.OwnerID, err))
					ids = append(ids, bean.ID)
					decryptionError = true
				}
			}

			if opts.AuthPasswordEncrypted != "" {
				if opts.AuthPassword, err = secret.DecryptSecret(oldEncryptionKey, opts.AuthPasswordEncrypted); err != nil {
					messages = append(messages, fmt.Sprintf("task.id=%d, task.doer_id=%d, task.repo_id=%d, task.owner_id=%d: secret.DecryptSecret(AuthPasswordEncrypted): %v", bean.ID, bean.DoerID, bean.RepoID, bean.OwnerID, err))
					ids = append(ids, bean.ID)
					decryptionError = true
				}
			}

			if opts.AuthTokenEncrypted != "" {
				if opts.AuthToken, err = secret.DecryptSecret(oldEncryptionKey, opts.AuthTokenEncrypted); err != nil {
					messages = append(messages, fmt.Sprintf("task.id=%d, task.doer_id=%d, task.repo_id=%d, task.owner_id=%d: secret.DecryptSecret(AuthTokenEncrypted): %v", bean.ID, bean.DoerID, bean.RepoID, bean.OwnerID, err))
					ids = append(ids, bean.ID)
					decryptionError = true
				}
			}

			// Don't migrate a task that has a decryption error.
			if decryptionError {
				return nil
			}

			if opts.CloneAddrEncrypted != "" {
				opts.CloneAddrEncrypted = base64.RawStdEncoding.EncodeToString(key.Encrypt([]byte(opts.CloneAddr), keying.ColumnAndJSONSelectorAndID("payload_content", "clone_addr_encrypted", bean.ID)))
			}

			if opts.AuthPasswordEncrypted != "" {
				opts.AuthPasswordEncrypted = base64.RawStdEncoding.EncodeToString(key.Encrypt([]byte(opts.AuthPassword), keying.ColumnAndJSONSelectorAndID("payload_content", "auth_password_encrypted", bean.ID)))
			}

			if opts.AuthTokenEncrypted != "" {
				opts.AuthTokenEncrypted = base64.RawStdEncoding.EncodeToString(key.Encrypt([]byte(opts.AuthToken), keying.ColumnAndJSONSelectorAndID("payload_content", "auth_token_encrypted", bean.ID)))
			}

			bs, err := json.Marshal(&opts)
			if err != nil {
				return err
			}
			bean.PayloadContent = string(bs)

			return taskUpdateCols(ctx, bean, "payload_content")
		})

		if err == nil {
			if len(ids) > 0 {
				log.Error("v14a_migrate_task_secrets: The following tasks were found to be corrupted and removed from the database.")
				for _, message := range messages {
					log.Error("v14a_migrate_task_secrets: %s", message)
				}

				_, err = sess.In("id", ids).NoAutoCondition().NoAutoTime().Delete(&Task{})
			}
		}
		return err
	})
}
