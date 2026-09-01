// Copyright 2026 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"testing"

	actions_model "forgejo.org/models/actions"
	"forgejo.org/models/unittest"
	notify_service "forgejo.org/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCancelAbandonedJobs(t *testing.T) {
	defer unittest.OverrideFixtures("services/actions/TestCancelAbandonedJobs")()
	require.NoError(t, unittest.PrepareTestDatabase())

	notifier := notify_service.NewMockNotifier(t)
	notifier.On("Run").Return().Maybe()
	notifier.On("WorkflowJobCompleted", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	notifier.On("WorkflowRunEvent", mock.Anything, mock.Anything).Return(nil)

	notify_service.RegisterNotifier(notifier)
	defer notify_service.UnregisterNotifier(notifier)

	require.NoError(t, CancelAbandonedJobs(t.Context()))

	// status waiting, too long, ready to be abandoned
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 600})
	assert.Equal(t, actions_model.StatusCancelled, job.Status)

	// status blocked, too long, ready to be abandoned
	job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 601})
	assert.Equal(t, actions_model.StatusCancelled, job.Status)

	// status blocked, *not* too long, not to be abandoned
	job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 602})
	assert.Equal(t, actions_model.StatusBlocked, job.Status)

	// status running, not to be abandoned
	job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 603})
	assert.Equal(t, actions_model.StatusRunning, job.Status)

	// related run needs approval, not to be abandoned
	job = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: 604})
	assert.Equal(t, actions_model.StatusWaiting, job.Status)

	notifier.AssertNumberOfCalls(t, "WorkflowJobCompleted", 2)
	notifier.AssertNumberOfCalls(t, "WorkflowRunEvent", 1)
	notifier.AssertCalled(
		t, "WorkflowJobCompleted", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == 600 && job.Status == actions_model.StatusCancelled
		}),
		actions_model.StatusWaiting,
	)
	notifier.AssertCalled(
		t, "WorkflowJobCompleted", mock.Anything,
		mock.MatchedBy(func(job *actions_model.ActionRunJob) bool {
			return job.ID == 601 && job.Status == actions_model.StatusCancelled
		}),
		actions_model.StatusBlocked,
	)
	notifier.AssertCalled(
		t, "WorkflowRunEvent", mock.Anything,
		mock.MatchedBy(func(event *actions_model.WorkflowRunCompleted) bool {
			return event.GetRun().ID == 900 && event.GetRun().Status == actions_model.StatusCancelled
		}),
	)
}
