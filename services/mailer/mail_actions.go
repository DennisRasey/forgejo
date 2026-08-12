// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package mailer

import (
	"bytes"

	actions_model "forgejo.org/models/actions"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/base"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/translation"
)

const (
	tplActionNowDone base.TplName = "actions/now_done"
)

var MailActionRun = mailActionRun // make it mockable
func mailActionRun(run *actions_model.ActionRun, priorStatus actions_model.Status) error {
	if setting.MailService == nil {
		// No mail service configured
		return nil
	}

	if !run.NotifyEmail {
		return nil
	}

	user := run.TriggerUser
	// this happens e.g. when this is a scheduled run
	if user.IsSystem() {
		user = run.Repo.Owner
	}
	if user.IsSystem() || user.Email == "" {
		return nil
	}

	if user.EmailNotificationsPreference == user_model.EmailNotificationsDisabled {
		return nil
	}

	return sendMailActionRun(user, run, priorStatus)
}

func sendMailActionRun(to *user_model.User, run *actions_model.ActionRun, priorStatus actions_model.Status) error {
	var (
		locale  = translation.NewLocale(to.Language)
		content bytes.Buffer
	)

	var subject string
	if run.Status.IsSuccess() {
		subject = locale.TrString("mail.actions.successful_run_after_failure_subject", run.Title, run.Repo.FullName())
	} else {
		subject = locale.TrString("mail.actions.not_successful_run_subject", run.Title, run.Repo.FullName())
	}

	commitSHA := run.CommitSHA
	if len(commitSHA) > 7 {
		commitSHA = commitSHA[:7]
	}

	data := map[string]any{
		"locale":          locale,
		"Link":            run.HTMLURL(),
		"Subject":         subject,
		"Language":        locale.Language(),
		"RepoFullName":    run.Repo.FullName(),
		"Run":             run,
		"TriggerUserLink": run.TriggerUser.HTMLURL(),
		"LastRun":         nil,
		"PriorStatus":     priorStatus,
		"CommitSHA":       commitSHA,
		"IsSuccess":       run.Status.IsSuccess(),
	}

	if err := bodyTemplates.ExecuteTemplate(&content, string(tplActionNowDone), data); err != nil {
		return err
	}

	msg := NewMessage(to.EmailTo(), subject, content.String())
	msg.Info = subject
	SendAsync(msg)

	return nil
}
