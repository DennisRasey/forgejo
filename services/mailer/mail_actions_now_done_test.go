// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package mailer

import (
	"slices"
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/db"
	org_model "forgejo.org/models/organization"
	repo_model "forgejo.org/models/repo"
	user_model "forgejo.org/models/user"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/test"
	notify_service "forgejo.org/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getActionsNowDoneTestUser(t *testing.T, name, email, notifications string) *user_model.User {
	t.Helper()
	user := new(user_model.User)
	user.Name = name
	user.Language = "en_US"
	user.IsAdmin = false
	user.Email = email
	user.LastLoginUnix = 1693648327
	user.CreatedUnix = 1693648027
	opts := user_model.CreateUserOverwriteOptions{
		AllowCreateOrganization:      optional.Some(true),
		EmailNotificationsPreference: &notifications,
	}
	require.NoError(t, user_model.AdminCreateUser(db.DefaultContext, user, &opts))
	return user
}

func getActionsNowDoneTestOrg(t *testing.T, name, email string, owner *user_model.User) *user_model.User {
	t.Helper()
	org := new(org_model.Organization)
	org.Name = name
	org.Language = "en_US"
	org.IsAdmin = false
	// contact email for the organization, for display purposes but otherwise not used as of v12
	org.Email = email
	org.LastLoginUnix = 1693648327
	org.CreatedUnix = 1693648027
	org.Email = email
	require.NoError(t, org_model.CreateOrganization(db.DefaultContext, org, owner))
	return (*user_model.User)(org)
}

func assertTranslatedLocaleMailActionsNowDone(t *testing.T, msgBody string) {
	AssertTranslatedLocale(t, msgBody, "mail.actions.successful_run_after_failure", "mail.actions.not_successful_run", "mail.actions.run_info_cur_status", "mail.actions.run_info_sha", "mail.actions.run_info_previous_status", "mail.actions.run_info_trigger", "mail.view_it_on")
}

func TestActionRunNowDoneStatusMatrix(t *testing.T) {
	successStatuses := []actions_model.Status{
		actions_model.StatusSuccess,
		actions_model.StatusSkipped,
		actions_model.StatusCancelled,
	}
	failureStatuses := []actions_model.Status{
		actions_model.StatusFailure,
	}

	for _, testCase := range []struct {
		name     string
		statuses []actions_model.Status
		run      bool
	}{
		{
			name:     "Failure",
			statuses: failureStatuses,
			run:      true,
		},
		{
			name:     "Success",
			statuses: successStatuses,
			run:      false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var called bool
			defer test.MockVariableValue(&MailActionRun, func(run *actions_model.ActionRun, priorStatus actions_model.Status) error {
				called = true
				return nil
			})()
			for _, status := range testCase.statuses {
				called = false
				n := NewNotifier()
				n.ActionRunNowDone(t.Context(), &actions_model.ActionRun{
					Status: status,
				}, actions_model.StatusUnknown)
				assert.Equal(t, testCase.run, called, "status = %s", status)
			}
		})
	}
}

func TestActionRunNowDoneNotificationMail(t *testing.T) {
	ctx := t.Context()

	defer test.MockVariableValue(&setting.Admin.DisableRegularOrgCreation, false)()

	actionsUser := user_model.NewActionsUser()
	require.NotEmpty(t, actionsUser.Email)

	repo := repo_model.Repository{
		Name:        "some repo",
		Description: "rockets are cool",
	}

	// Do some funky stuff with the action run's ids:
	// The run with the larger ID finished first.
	// This is odd but something that must work.
	run1 := &actions_model.ActionRun{ID: 2, Repo: &repo, RepoID: repo.ID, Title: "some workflow", Status: actions_model.StatusFailure, Stopped: 1745821796, TriggerEvent: "workflow_dispatch"}
	run2 := &actions_model.ActionRun{ID: 1, Repo: &repo, RepoID: repo.ID, Title: "some workflow", Status: actions_model.StatusSuccess, Stopped: 1745822796, TriggerEvent: "push"}

	assignUsers := func(triggerUser, owner *user_model.User) {
		for _, run := range []*actions_model.ActionRun{run1, run2} {
			run.TriggerUser = triggerUser
			run.TriggerUserID = triggerUser.ID
			run.NotifyEmail = true
		}
		repo.Owner = owner
		repo.OwnerID = owner.ID
	}

	notify_service.RegisterNotifier(NewNotifier())

	orgOwner := getActionsNowDoneTestUser(t, "org_owner", "org_owner@example.com", "disabled")
	defer CleanUpUsers(ctx, []*user_model.User{orgOwner})

	t.Run("DontSendNotificationEmailOnFirstActionSuccess", func(t *testing.T) {
		user := getActionsNowDoneTestUser(t, "new_user", "new_user@example.com", "enabled")
		defer CleanUpUsers(ctx, []*user_model.User{user})
		assignUsers(user, user)
		defer MockMailSettings(func(msgs ...*Message) {
			assert.Fail(t, "no mail should be sent")
		})()
		notify_service.ActionRunNowDone(ctx, run2, actions_model.StatusRunning)
	})

	t.Run("WorkflowEnableEmailNotificationIsFalse", func(t *testing.T) {
		user := getActionsNowDoneTestUser(t, "new_user1", "new_user1@example.com", "enabled")
		defer CleanUpUsers(ctx, []*user_model.User{user})
		assignUsers(user, user)
		defer MockMailSettings(func(msgs ...*Message) {
			assert.Fail(t, "no mail should be sent")
		})()
		run2.NotifyEmail = false
		notify_service.ActionRunNowDone(ctx, run2, actions_model.StatusRunning)
	})

	for _, testCase := range []struct {
		name        string
		triggerUser *user_model.User
		owner       *user_model.User
		expected    string
		expectMail  bool
	}{
		{
			// if the action is assigned a trigger user in a repository
			// owned by a regular user, the mail is sent to the trigger user
			name:        "RegularTriggerUser",
			triggerUser: getActionsNowDoneTestUser(t, "new_trigger_user0", "new_trigger_user0@example.com", user_model.EmailNotificationsEnabled),
			owner:       getActionsNowDoneTestUser(t, "new_owner0", "new_owner0@example.com", user_model.EmailNotificationsEnabled),
			expected:    "trigger",
			expectMail:  true,
		},
		{
			// if the action is assigned to a system user (e.g. ActionsUser)
			// in a repository owned by a regular user, the mail is sent to
			// the user that owns the repository
			name:        "SystemTriggerUserAndRegularOwner",
			triggerUser: actionsUser,
			owner:       getActionsNowDoneTestUser(t, "new_owner1", "new_owner1@example.com", user_model.EmailNotificationsEnabled),
			expected:    "owner",
			expectMail:  true,
		},
		{
			// if the action is assigned a trigger user with disabled notifications in a repository
			// owned by a regular user, no mail is sent
			name:        "RegularTriggerUserNotificationsDisabled",
			triggerUser: getActionsNowDoneTestUser(t, "new_trigger_user2", "new_trigger_user2@example.com", user_model.EmailNotificationsDisabled),
			owner:       getActionsNowDoneTestUser(t, "new_owner2", "new_owner2@example.com", user_model.EmailNotificationsEnabled),
			expectMail:  false,
		},
		{
			// if the action is assigned to a system user (e.g. ActionsUser)
			// owned by a regular user with disabled notifications, no mail is sent
			name:        "SystemTriggerUserAndRegularOwnerNotificationsDisabled",
			triggerUser: actionsUser,
			owner:       getActionsNowDoneTestUser(t, "new_owner3", "new_owner3@example.com", user_model.EmailNotificationsDisabled),
			expectMail:  false,
		},
		{
			// if the action is assigned to a system user (e.g. ActionsUser)
			// in a repository owned by an organization with an email contact, the mail is sent to
			// this email contact
			name:        "SystemTriggerUserAndOrgOwner",
			triggerUser: actionsUser,
			owner:       getActionsNowDoneTestOrg(t, "new_org1", "new_org_owner0@example.com", orgOwner),
			expected:    "owner",
			expectMail:  true,
		},
		{
			// if the action is assigned to a system user (e.g. ActionsUser)
			// in a repository owned by an organization without an email contact, no mail is sent
			name:        "SystemTriggerUserAndNoMailOrgOwner",
			triggerUser: actionsUser,
			owner:       getActionsNowDoneTestOrg(t, "new_org2", "", orgOwner),
			expectMail:  false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assignUsers(testCase.triggerUser, testCase.owner)
			defer CleanUpUsers(ctx, slices.DeleteFunc([]*user_model.User{testCase.triggerUser, testCase.owner}, func(user *user_model.User) bool {
				return user.IsSystem()
			}))

			t.Run("SendNotificationEmailOnActionRunFailed", func(t *testing.T) {
				mailSent := false
				defer MockMailSettings(func(msgs ...*Message) {
					assert.Len(t, msgs, 1)
					msg := msgs[0]
					assert.False(t, mailSent, "sent mail twice")
					expectedEmail := testCase.triggerUser.Email
					if testCase.expected == "owner" { // otherwise "trigger"
						expectedEmail = testCase.owner.Email
					}
					require.Contains(t, msg.To, expectedEmail, "sent mail to unknown sender")
					mailSent = true
					assert.Contains(t, msg.Body, testCase.triggerUser.HTMLURL())
					assert.Contains(t, msg.Body, testCase.triggerUser.Name)
					// what happened
					assert.Contains(t, msg.Body, "failed")
					// new status of run
					assert.Contains(t, msg.Body, "failure")
					// prior status of this run
					assert.Contains(t, msg.Body, "waiting")
					assertTranslatedLocaleMailActionsNowDone(t, msg.Body)
				})()
				require.NotNil(t, setting.MailService)

				notify_service.ActionRunNowDone(ctx, run1, actions_model.StatusWaiting)
				assert.Equal(t, testCase.expectMail, mailSent)
			})
		})
	}
}
